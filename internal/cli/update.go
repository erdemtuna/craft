package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/fetch"
	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/erdemtuna/craft/internal/semver"
	"github.com/erdemtuna/craft/internal/ui"
	"github.com/spf13/cobra"
)

var updateTarget string
var updateDryRun bool

var updateCmd = &cobra.Command{
	Use:   "update [alias]",
	Short: "Update dependencies to latest versions",
	Long: `Re-resolve dependencies to latest available versions. Updates craft.yaml and craft.pin.yaml.

For tagged deps: finds latest semver tag via MVS.
For branch-tracked deps: re-resolves to latest branch HEAD commit.
For commit-pinned deps: skipped (commit pins are deliberate freezes).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().StringVar(&updateTarget, "target", "", "Override agent auto-detection with a custom install path")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be updated without making changes")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	var root, manifestPath, pfPath string
	var err error

	if globalFlag {
		manifestPath, err = GlobalManifestPath()
		if err != nil {
			return err
		}
		pfPath, err = GlobalPinfilePath()
		if err != nil {
			return err
		}
	} else {
		root, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		manifestPath = filepath.Join(root, "craft.yaml")
		pfPath = filepath.Join(root, "craft.pin.yaml")
	}

	progress := ui.NewProgress()

	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if globalFlag {
				return fmt.Errorf("no global skills installed\n  hint: use `craft get` to install skills")
			}
			return fmt.Errorf("craft.yaml not found\n  hint: run `craft init` to create one")
		}
		return fmt.Errorf("reading craft.yaml: %w", err)
	}

	if len(m.Dependencies) == 0 {
		cmd.Println("No dependencies to update.")
		return nil
	}

	// Set up cache and fetcher
	fetcher, err := newFetcher()
	if err != nil {
		return err
	}

	// Determine which deps to update
	var targetAlias string
	if len(args) > 0 {
		targetAlias = args[0]
		if _, ok := m.Dependencies[targetAlias]; !ok {
			return fmt.Errorf("dependency %q not found in craft.yaml\n  hint: available aliases: %s",
				targetAlias, availableAliases(m.Dependencies))
		}
	}

	// Load existing pinfile (needed for branch update comparison)
	var existingPinfile *pinfile.Pinfile
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
	}

	// Find latest versions for targeted deps
	progress.Start("Checking for updates...")
	updated := false
	for alias, depURL := range m.Dependencies {
		if targetAlias != "" && alias != targetAlias {
			continue
		}

		parsed, err := resolve.ParseDepURL(depURL)
		if err != nil {
			return fmt.Errorf("invalid dependency URL for %q: %w", alias, err)
		}

		switch parsed.RefType {
		case resolve.RefTypeCommit:
			// Commit pins are deliberate freezes — skip silently
			continue

		case resolve.RefTypeBranch:
			// Re-resolve branch HEAD to detect changes
			cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())
			commitSHA, err := fetcher.ResolveRef(cloneURL, parsed.GitRef())
			if err != nil {
				return fmt.Errorf("resolving branch %q for %s: %w\n  hint: check if the branch still exists", parsed.Ref, depURL, err)
			}

			// Compare against existing pinfile to detect changes
			if existingPinfile != nil {
				if entry, ok := existingPinfile.Resolved[depURL]; ok {
					if entry.Commit == commitSHA {
						continue // No change
					}
				}
			}
			// Branch HEAD changed — force re-resolution
			updated = true
			short := commitSHA
			if len(short) > 12 {
				short = short[:12]
			}
			cmd.Printf("  %s: branch:%s → %s\n", alias, parsed.Ref, short)

		case resolve.RefTypeTag:
			cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())

			tags, err := fetcher.ListTags(cloneURL)
			if err != nil {
				return fmt.Errorf("listing tags for %s: %w\n  hint: check your connection or set GITHUB_TOKEN for private repos", depURL, err)
			}

			latest := semver.FindLatest(tags)
			if latest == "" {
				cmd.PrintErrf("warning: no semver tags found for %s\n", parsed.PackageIdentity())
				continue
			}

			if semver.Compare(strings.TrimPrefix(latest, "v"), parsed.Version) > 0 {
				newURL := parsed.WithVersion(latest)
				m.Dependencies[alias] = newURL
				updated = true
				cmd.Printf("  %s: %s → %s\n", alias, parsed.GitRef(), latest)
			}
		}
	}

	if !updated {
		msg := "All dependencies are up to date."
		progress.Done(msg)
		if !progress.IsTTY() {
			cmd.Println(msg)
		}
		return nil
	}

	// Resolve before writing anything to disk
	progress.Update("Resolving updated dependencies...")
	forceResolve := make(map[string]bool)
	for alias, depURL := range m.Dependencies {
		if targetAlias == "" || alias == targetAlias {
			forceResolve[depURL] = true
		}
	}

	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
		ForceResolve:    forceResolve,
	})
	if err != nil {
		progress.Fail("Resolution failed")
		return fmt.Errorf("resolution failed: %w", err)
	}

	// Dry-run: show what would change and exit
	if updateDryRun {
		progress.Done("Dry-run complete")
		printDryRunSummary(cmd, result, "~")
		return nil
	}

	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Write updated manifest only after resolution and pinfile write succeed
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}

	// Install
	progress.Update("Installing skills...")
	if globalFlag {
		// Global: install to agent directories
		targetPaths, err := resolveInstallTargets(updateTarget)
		if err != nil {
			return err
		}

		skillFiles, err := collectSkillFiles(fetcher, result)
		if err != nil {
			return err
		}

		for _, targetPath := range targetPaths {
			if err := installlib.InstallFlat(targetPath, skillFiles); err != nil {
				progress.Fail("Installation failed")
				return fmt.Errorf("installation failed: %w", err)
			}
		}

		skillCount := countSkills(result)
		msg := fmt.Sprintf("Updated and installed %d skill(s) to %s", skillCount, strings.Join(targetPaths, ", "))
		progress.Done(msg)
		if !progress.IsTTY() {
			cmd.Println(msg)
		}
	} else {
		// Project: vendor to forge/
		if updateTarget != "" {
			return fmt.Errorf("--target is not supported for project updates (skills vendor to forge/)\n  hint: use `craft update -g --target %s` for global update to a custom path", updateTarget)
		}
		forgePath := filepath.Join(root, "forge")

		skillFiles, err := collectSkillFiles(fetcher, result)
		if err != nil {
			return err
		}

		if err := installlib.Install(forgePath, skillFiles); err != nil {
			progress.Fail("Vendoring failed")
			return fmt.Errorf("vendoring failed: %w", err)
		}

		if err := ensureGitignore(root, "forge/"); err != nil {
			cmd.PrintErrf("warning: could not update .gitignore: %v\n", err)
		}

		skillCount := countSkills(result)
		msg := fmt.Sprintf("Updated and vendored %d skill(s) to forge/", skillCount)
		progress.Done(msg)
		if !progress.IsTTY() {
			cmd.Println(msg)
		}
	}

	// Print dependency tree to stderr
	printDependencyTree(cmd, m, result)

	return nil
}

func writeManifestAtomic(path string, m *manifest.Manifest) error {
	return writeAtomic(path, func(w io.Writer) error {
		return manifest.Write(m, w)
	})
}

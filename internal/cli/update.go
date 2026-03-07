package cli

import (
	"errors"
	"fmt"
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

var updateCmd = &cobra.Command{
	Use:   "update [alias]",
	Short: "Update dependencies to latest versions",
	Long:  "Re-resolve dependencies to latest available semver tags. Updates craft.yaml and craft.pin.yaml.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().StringVar(&updateTarget, "target", "", "Override agent auto-detection with a custom install path")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	progress := ui.NewProgress()

	manifestPath := filepath.Join(root, "craft.yaml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

		if latest != parsed.GitTag() {
			newURL := parsed.WithVersion(latest)
			m.Dependencies[alias] = newURL
			updated = true
			cmd.Printf("  %s: %s → %s\n", alias, parsed.GitTag(), latest)
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

	// Resolve before writing anything to disk — if resolution fails,
	// neither manifest nor pinfile is modified.
	progress.Update("Resolving updated dependencies...")
	forceResolve := make(map[string]bool)
	for alias, depURL := range m.Dependencies {
		if targetAlias == "" || alias == targetAlias {
			forceResolve[depURL] = true
		}
	}

	// Load existing pinfile
	var existingPinfile *pinfile.Pinfile
	pfPath := filepath.Join(root, "craft.pin.yaml")
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
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

	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Write updated manifest only after resolution and pinfile write succeed
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}

	// Install
	progress.Update("Installing skills...")
	targetPaths, err := resolveInstallTargets(updateTarget)
	if err != nil {
		return err
	}

	skillFiles, err := collectSkillFiles(fetcher, result)
	if err != nil {
		return err
	}

	for _, targetPath := range targetPaths {
		if err := installlib.Install(targetPath, skillFiles); err != nil {
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

	// Print dependency tree to stderr
	printDependencyTree(cmd, m, result)

	return nil
}

func writeManifestAtomic(path string, m *manifest.Manifest) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating manifest: %w", err)
	}

	if err := manifest.Write(m, f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing manifest: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing manifest: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}



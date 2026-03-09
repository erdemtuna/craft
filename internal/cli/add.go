package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/spf13/cobra"
)

var addInstall bool

var addCmd = &cobra.Command{
	Use:   "add [alias] <url>",
	Short: "Add a dependency",
	Long: `Add a dependency to craft.yaml. The URL must be in one of these formats:

  host/org/repo@vMAJOR.MINOR.PATCH   (tagged version)
  host/org/repo@<commit-sha>         (commit pin, ≥7 hex chars)
  host/org/repo@branch:<name>        (branch tracking)

If no alias is provided, one is derived from the repository name.
The dependency is verified by resolving it before updating the manifest.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().BoolVar(&addInstall, "install", false, "Run install after adding the dependency")
	addCmd.Flags().StringVar(&installTarget, "target", "", "Override agent auto-detection with a custom install path (used with --install)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	var alias, depURL string
	if len(args) == 2 {
		alias = args[0]
		depURL = args[1]
	} else {
		depURL = args[0]
	}

	// Validate URL format
	parsed, err := resolve.ParseDepURL(depURL)
	if err != nil {
		return fmt.Errorf("%w\n  hint: expected format: host/org/repo@v1.0.0, host/org/repo@<sha>, or host/org/repo@branch:<name>", err)
	}

	// Warn about non-tagged dependencies
	if parsed.RefType != resolve.RefTypeTag {
		cmd.PrintErrln("⚠ Non-tagged dependency: " + depURL)
		if parsed.RefType == resolve.RefTypeBranch {
			cmd.PrintErrln("  Branch-tracked deps have weaker reproducibility guarantees.")
		} else {
			cmd.PrintErrln("  Commit-pinned deps are reproducible but frozen; no updates available.")
		}
	}

	// Derive alias from repo name if not provided
	if alias == "" {
		alias = parsed.Repo
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Parse existing manifest
	manifestPath := filepath.Join(root, "craft.yaml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("craft.yaml not found\n  hint: run `craft init` to create one")
		}
		return fmt.Errorf("reading craft.yaml: %w", err)
	}

	// Check for existing dependency
	isUpdate := false
	if existing, ok := m.Dependencies[alias]; ok {
		isUpdate = true
		if existing == depURL {
			cmd.Printf("Dependency %q is already at %s — nothing to do.\n", alias, depURL)
			return nil
		}
		cmd.Printf("Updating %q: %s → %s\n", alias, existing, depURL)
	}

	// Add dependency in memory
	if m.Dependencies == nil {
		m.Dependencies = make(map[string]string)
	}
	m.Dependencies[alias] = depURL

	// Validate by resolving with full manifest
	fetcher, err := newFetcher()
	if err != nil {
		return err
	}

	// Load existing pinfile to preserve pins for unchanged deps
	var existingPinfile *pinfile.Pinfile
	pfPath := filepath.Join(root, "craft.pin.yaml")
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
	}

	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
		ForceResolve:    map[string]bool{depURL: true},
	})
	if err != nil {
		return fmt.Errorf("dependency verification failed: %w", err)
	}

	// Find the added dependency in results for summary
	var addedSkills []string
	for _, dep := range result.Resolved {
		if dep.Alias == alias {
			addedSkills = dep.Skills
			break
		}
	}

	// Write manifest atomically
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}

	// Print summary
	if isUpdate {
		cmd.Printf("Updated %q → %s\n", alias, depURL)
	} else {
		cmd.Printf("Added %q → %s\n", alias, depURL)
	}
	if len(addedSkills) > 0 {
		cmd.Printf("  skills: %s\n", strings.Join(addedSkills, ", "))
	}
	// Print ref-type-appropriate summary
	switch parsed.RefType {
	case resolve.RefTypeTag:
		cmd.Printf("  version: %s\n", parsed.GitRef())
	case resolve.RefTypeCommit:
		ref := parsed.Ref
		if len(ref) > 12 {
			ref = ref[:12]
		}
		cmd.Printf("  commit: %s\n", ref)
	case resolve.RefTypeBranch:
		cmd.Printf("  branch: %s\n", parsed.Ref)
	}

	// Optionally run install
	if addInstall {
		// Write pinfile
		pfPath := filepath.Join(root, "craft.pin.yaml")
		if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
			return err
		}

		targetPaths, err := resolveInstallTargets(installTarget)
		if err != nil {
			return err
		}

		skillFiles, err := collectSkillFiles(fetcher, result)
		if err != nil {
			return err
		}

		for _, targetPath := range targetPaths {
			if err := installlib.Install(targetPath, skillFiles); err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}
		}

		cmd.Printf("Installed %d skill(s) to %s\n", countSkills(result), strings.Join(targetPaths, ", "))
	}

	return nil
}

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
	"github.com/erdemtuna/craft/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	addCmd.Flags().BoolVar(&addInstall, "install", false, "Run install after adding the dependency (vendors to forge/)")
	addCmd.Flags().Bool("all", false, "Install all skills without interactive selection")
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
		if existing.URL == depURL {
			cmd.Printf("Dependency %q is already at %s — nothing to do.\n", alias, depURL)
			return nil
		}
		cmd.Printf("Updating %q: %s → %s\n", alias, existing.URL, depURL)
	}

	// Add dependency in memory
	if m.Dependencies == nil {
		m.Dependencies = make(map[string]manifest.DependencySpec)
	}
	m.Dependencies[alias] = manifest.DependencySpec{URL: depURL}

	// Interactive skill selection: if running in a TTY with multiple
	// skills available, let the user pick which skills to include.
	addAll, _ := cmd.Flags().GetBool("all")
	if !addAll && term.IsTerminal(int(os.Stdin.Fd())) {
		selected, err := discoverAndSelect(cmd, parsed)
		if err != nil {
			cmd.PrintErrf("⚠ Could not discover skills: %v — installing all.\n", err)
		} else if len(selected) > 0 {
			m.Dependencies[alias] = manifest.DependencySpec{URL: depURL, Select: selected}
		}
	}

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

	// Optionally run install (vendor to forge/)
	if addInstall {
		// Write pinfile
		pfPath := filepath.Join(root, "craft.pin.yaml")
		if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
			return err
		}

		forgePath := filepath.Join(root, "forge")

		skillFiles, err := collectSkillFiles(fetcher, result)
		if err != nil {
			return err
		}

		if err := installlib.Install(forgePath, skillFiles); err != nil {
			return fmt.Errorf("vendoring failed: %w", err)
		}

		// Auto-add forge/ to .gitignore
		if err := ensureGitignore(root, "forge/"); err != nil {
			cmd.PrintErrf("warning: could not update .gitignore: %v\n", err)
		}

		cmd.Printf("Vendored %d skill(s) to forge/\n", countSkills(result))
	}

	return nil
}

// discoverAndSelect fetches the skill list for a package and prompts the
// user to pick a subset when multiple skills are found. Returns nil (meaning
// "all") when the user selects everything or when only one skill exists.
// Returns the selected skill directory paths when a subset is chosen.
func discoverAndSelect(cmd *cobra.Command, parsed *resolve.DepURL) ([]string, error) {
	fetcher, err := newFetcher()
	if err != nil {
		return nil, err
	}

	cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())

	// Resolve the ref to a commit SHA
	var commitSHA string
	if parsed.RefType == resolve.RefTypeCommit {
		commitSHA = parsed.Ref
	} else {
		commitSHA, err = fetcher.ResolveRef(cloneURL, parsed.GitRef())
		if err != nil {
			return nil, fmt.Errorf("resolving ref: %w", err)
		}
	}

	// List all files and discover skills
	paths, err := fetcher.ListTree(cloneURL, commitSHA)
	if err != nil {
		return nil, fmt.Errorf("listing tree: %w", err)
	}

	skills, err := resolve.DiscoverSkills(paths, func(path string) ([]byte, error) {
		files, err := fetcher.ReadFiles(cloneURL, commitSHA, []string{path})
		if err != nil {
			return nil, err
		}
		content, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return content, nil
	})
	if err != nil {
		return nil, fmt.Errorf("discovering skills: %w", err)
	}

	if len(skills) <= 1 {
		return nil, nil
	}

	// Build display names
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}

	cmd.Printf("Found %d skills in package:\n", len(skills))
	indices, err := ui.MultiSelect("Select skills to include:", names, cmd.OutOrStderr(), os.Stdin)
	if err != nil {
		return nil, err
	}

	// nil means "all"
	if indices == nil {
		return nil, nil
	}

	// Map selected indices to skill directory paths
	selected := make([]string, len(indices))
	for i, idx := range indices {
		selected[i] = skills[idx].Dir
	}
	return selected, nil
}

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/spf13/cobra"
)

// requireManifestAndPinfile parses both craft.yaml and craft.pin.yaml from the
// current working directory. It returns a user-friendly error if either file
// is missing — callers should not proceed without both files.
func requireManifestAndPinfile() (*manifest.Manifest, *pinfile.Pinfile, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("getting working directory: %w", err)
	}

	m, err := manifest.ParseFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("craft.yaml not found\n  hint: run `craft init` to create one")
		}
		return nil, nil, fmt.Errorf("reading craft.yaml: %w", err)
	}

	pf, err := pinfile.ParseFile(filepath.Join(root, "craft.pin.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("craft.pin.yaml not found\n  hint: run `craft install` to resolve and pin dependencies")
		}
		return nil, nil, fmt.Errorf("reading craft.pin.yaml: %w", err)
	}

	return m, pf, nil
}

// printDryRunSummary prints what would be resolved without making changes.
// Used by both install --dry-run and update --dry-run.
func printDryRunSummary(cmd *cobra.Command, result *resolve.ResolveResult, prefix string) {
	cmd.Printf("Would resolve %d dependency(ies):\n", len(result.Resolved))
	for _, dep := range result.Resolved {
		skillWord := "skills"
		if len(dep.Skills) == 1 {
			skillWord = "skill"
		}
		parsed, err := resolve.ParseDepURL(dep.URL)
		if err != nil {
			cmd.Printf("  %s %s  (%d %s)\n", prefix, sanitize(dep.Alias), len(dep.Skills), skillWord)
			continue
		}
		if len(dep.Skills) > 0 {
			cmd.Printf("  %s %s  %s  (%d %s: %s)\n", prefix, sanitize(dep.Alias), parsed.GitRef(),
				len(dep.Skills), skillWord, sanitize(strings.Join(dep.Skills, ", ")))
		} else {
			cmd.Printf("  %s %s  %s  (0 skills)\n", prefix, sanitize(dep.Alias), parsed.GitRef())
		}
	}
	cmd.Println("\nNo changes made.")
}

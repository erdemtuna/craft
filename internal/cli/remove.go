package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/spf13/cobra"
)

var removeTarget string

var removeCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a dependency",
	Long:  "Remove a dependency from craft.yaml, update craft.pin.yaml, and clean up orphaned skills from the install target.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	removeCmd.Flags().StringVar(&removeTarget, "target", "", "Override agent auto-detection with a custom install path (for cleanup)")
}

func runRemove(cmd *cobra.Command, args []string) error {
	alias := args[0]

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Parse manifest
	manifestPath := filepath.Join(root, "craft.yaml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("craft.yaml not found\n  hint: run `craft init` to create one")
		}
		return fmt.Errorf("reading craft.yaml: %w", err)
	}

	// Check alias exists
	depURL, ok := m.Dependencies[alias]
	if !ok {
		available := availableAliases(m.Dependencies)
		return fmt.Errorf("dependency %q not found in craft.yaml\n  hint: available aliases: %s", alias, available)
	}

	// Identify skills from the removed dependency (from pinfile)
	pfPath := filepath.Join(root, "craft.pin.yaml")
	pf, pfErr := pinfile.ParseFile(pfPath)

	var removedSkills []string
	if pfErr == nil {
		// Find the pinfile entry for this dependency
		if entry, ok := pf.Resolved[depURL]; ok {
			removedSkills = entry.Skills
		}
	}

	// Remove from manifest
	delete(m.Dependencies, alias)

	// Write updated manifest
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}

	cmd.Printf("Removed %q (%s)\n", alias, depURL)

	// Update pinfile
	if pfErr == nil {
		// Collect all skills still needed by remaining dependencies
		remainingSkills := make(map[string]bool)
		for _, remainingURL := range m.Dependencies {
			if entry, ok := pf.Resolved[remainingURL]; ok {
				for _, s := range entry.Skills {
					remainingSkills[s] = true
				}
			}
		}

		// Remove the dep entry from pinfile
		delete(pf.Resolved, depURL)

		// Write updated pinfile
		if err := writePinfileAtomic(pfPath, pf); err != nil {
			return err
		}

		// Find orphaned skills (only in removed dep, not in any remaining dep)
		var orphaned []string
		for _, s := range removedSkills {
			if !remainingSkills[s] {
				orphaned = append(orphaned, s)
			}
		}

		// Clean up orphaned skills from install target
		if len(orphaned) > 0 {
			targetPath, err := resolveInstallTargets(removeTarget)
			if err != nil {
				// If we can't determine target, just report what was orphaned
				cmd.Printf("  orphaned skills (manual cleanup needed): %s\n", strings.Join(orphaned, ", "))
				return nil
			}

			var cleaned []string
			for _, skillName := range orphaned {
				removed := false
				for _, tp := range targetPath {
					skillDir := filepath.Join(tp, skillName)
					// Path traversal protection
					absSkillDir, err := filepath.Abs(skillDir)
					if err != nil {
						continue
					}
					absTarget, err := filepath.Abs(tp)
					if err != nil {
						continue
					}
					if !strings.HasPrefix(absSkillDir, absTarget+string(filepath.Separator)) {
						cmd.PrintErrf("  warning: skill name %q escapes target directory, skipping\n", skillName)
						continue
					}

					if _, err := os.Stat(skillDir); err == nil {
						if err := os.RemoveAll(skillDir); err != nil {
							cmd.PrintErrf("  warning: could not remove %s: %v\n", skillDir, err)
						} else {
							removed = true
						}
					}
				}
				if removed {
					cleaned = append(cleaned, skillName)
				}
			}

			if len(cleaned) > 0 {
				cmd.Printf("  cleaned up %d orphaned skill(s): %s\n", len(cleaned), strings.Join(cleaned, ", "))
			}
		}
	}

	return nil
}

// availableAliases formats the available dependency aliases for error messages.
func availableAliases(deps map[string]string) string {
	if len(deps) == 0 {
		return "(none)"
	}
	aliases := make([]string, 0, len(deps))
	for k := range deps {
		aliases = append(aliases, k)
	}
	sort.Strings(aliases)
	return strings.Join(aliases, ", ")
}

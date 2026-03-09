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
	"github.com/erdemtuna/craft/internal/resolve"
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

	var manifestPath, pfPath string
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
		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		manifestPath = filepath.Join(root, "craft.yaml")
		pfPath = filepath.Join(root, "craft.pin.yaml")
	}

	// Parse manifest
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

	// Check alias exists
	depURL, ok := m.Dependencies[alias]
	if !ok {
		available := availableAliases(m.Dependencies)
		return fmt.Errorf("dependency %q not found in craft.yaml\n  hint: available aliases: %s", alias, available)
	}

	// Identify skills from the removed dependency (from pinfile)
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
		// Remove the dep entry from pinfile
		delete(pf.Resolved, depURL)

		// Write updated pinfile
		if err := writePinfileAtomic(pfPath, pf); err != nil {
			return err
		}

		// With namespaced paths, every skill from the removed dep has a
		// unique disk path (host/owner/repo/skill), so all are orphaned.
		orphaned := removedSkills

		// Clean up orphaned skills from install target
		if len(orphaned) > 0 {
			var targetPath []string
			if globalFlag {
				// Global: clean from agent directories
				targetPath, err = resolveInstallTargets(removeTarget)
			} else if removeTarget != "" {
				// Project with explicit --target: use that path
				targetPath = []string{removeTarget}
			} else {
				// Project: clean from forge/ directory
				root, err := os.Getwd()
				if err == nil {
					targetPath = []string{filepath.Join(root, "forge")}
				}
			}
			if err != nil {
				// If we can't determine target, just report what was orphaned
				cmd.Printf("  orphaned skills (manual cleanup needed): %s\n", strings.Join(orphaned, ", "))
				return nil
			}

			// Parse dep URL to get namespace prefix (host/owner/repo)
			parsed, parseErr := resolve.ParseDepURL(depURL)
			if parseErr != nil {
				cmd.PrintErrf("  warning: could not parse dep URL %q for cleanup: %v\n", depURL, parseErr)
				cmd.Printf("  orphaned skills (manual cleanup needed): %s\n", strings.Join(orphaned, ", "))
				return nil
			}
			nsPrefix := parsed.PackageIdentity()

			var cleaned []string
			for _, skillName := range orphaned {
				removedFromAny := false
				for _, tp := range targetPath {
					skillDir := filepath.Join(tp, nsPrefix, skillName)
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
							removedFromAny = true
							cleanEmptyParents(tp, filepath.Dir(skillDir))
						}
					}
				}
				if removedFromAny {
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

// cleanEmptyParents removes empty directories from dir up to (but not
// including) root. Uses os.Remove which fails on non-empty dirs — safe.
func cleanEmptyParents(root, dir string) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return
	}
	for {
		absDir, err := filepath.Abs(dir)
		if err != nil || absDir == absRoot || !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) {
			break
		}
		if err := os.Remove(dir); err != nil {
			break // not empty or permission error
		}
		dir = filepath.Dir(dir)
	}
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

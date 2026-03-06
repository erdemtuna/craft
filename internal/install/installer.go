// Package install copies resolved skill directories to the target path.
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Install copies skill files to the target directory as <target>/<skill-name>/.
// Each entry in skills maps skill name to a map of relative file paths to contents.
// Files are written to a staging directory first and swapped into place to avoid
// leaving a skill in a partially-installed state if the process is interrupted.
func Install(target string, skills map[string]map[string][]byte) error {
	if err := os.MkdirAll(target, 0o700); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolving target path: %w", err)
	}

	for skillName, files := range skills {
		skillDir := filepath.Join(target, skillName)
		absSkillDir, err := filepath.Abs(skillDir)
		if err != nil {
			return fmt.Errorf("resolving skill path: %w", err)
		}
		if !strings.HasPrefix(absSkillDir, absTarget+string(filepath.Separator)) {
			return fmt.Errorf("skill name %q escapes target directory", skillName)
		}

		// Write to staging directory for atomicity
		stagingDir := skillDir + ".staging"
		// Clean up any leftover staging directory from a previous interrupted install
		os.RemoveAll(stagingDir)

		if err := os.MkdirAll(stagingDir, 0o700); err != nil {
			return fmt.Errorf("creating staging directory for %q: %w", skillName, err)
		}

		// Write all files to staging directory
		writeErr := func() error {
			for relPath, content := range files {
				fullPath := filepath.Join(stagingDir, relPath)
				absFullPath, err := filepath.Abs(fullPath)
				if err != nil {
					return fmt.Errorf("resolving file path: %w", err)
				}
				// Validate against the staging dir (same structure as final)
				if !strings.HasPrefix(absFullPath, filepath.Clean(stagingDir)+string(filepath.Separator)) {
					return fmt.Errorf("file path %q escapes skill directory", relPath)
				}

				if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
					return fmt.Errorf("creating directory for %q: %w", relPath, err)
				}

				if err := os.WriteFile(fullPath, content, 0o644); err != nil {
					return fmt.Errorf("writing %q: %w", relPath, err)
				}
			}
			return nil
		}()

		if writeErr != nil {
			os.RemoveAll(stagingDir)
			return writeErr
		}

		// Atomic swap: remove old, rename staging to final
		if err := os.RemoveAll(skillDir); err != nil {
			os.RemoveAll(stagingDir)
			return fmt.Errorf("removing existing skill %q: %w", skillName, err)
		}

		if err := os.Rename(stagingDir, skillDir); err != nil {
			os.RemoveAll(stagingDir)
			return fmt.Errorf("installing skill %q: %w", skillName, err)
		}
	}

	return nil
}

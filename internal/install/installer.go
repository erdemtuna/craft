// Package install copies resolved skill directories to the target path.
package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// Install copies skill files to the target directory as <target>/<skill-name>/.
// Each entry in skills maps skill name to a map of relative file paths to contents.
func Install(target string, skills map[string]map[string][]byte) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	for skillName, files := range skills {
		skillDir := filepath.Join(target, skillName)

		// Remove existing skill directory (overwrite)
		if err := os.RemoveAll(skillDir); err != nil {
			return fmt.Errorf("removing existing skill %q: %w", skillName, err)
		}

		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("creating skill directory %q: %w", skillName, err)
		}

		for relPath, content := range files {
			fullPath := filepath.Join(skillDir, relPath)

			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("creating directory for %q: %w", relPath, err)
			}

			if err := os.WriteFile(fullPath, content, 0o644); err != nil {
				return fmt.Errorf("writing %q: %w", relPath, err)
			}
		}
	}

	return nil
}

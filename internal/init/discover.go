// Package initcmd provides the craft init command logic including
// skill directory auto-discovery and interactive manifest creation.
package initcmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skipDirs contains directory names to skip during skill discovery.
// Per spec FR-015: .git, .paw, node_modules. Hidden directories (starting
// with '.') are also skipped via a separate check in the walk function.
var skipDirs = map[string]bool{
	".git":         true,
	".paw":         true,
	"node_modules": true,
}

// DiscoverSkills recursively walks the directory tree from root,
// finding directories that contain a SKILL.md file. Returns relative
// paths sorted alphabetically. Skips hidden directories (starting with '.')
// and standard infrastructure directories.
func DiscoverSkills(root string) ([]string, error) {
	var skills []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible directories
		}

		if !d.IsDir() {
			return nil
		}

		// Get path relative to root
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Skip the root itself
		if rel == "." {
			return nil
		}

		name := d.Name()

		// Skip hidden directories (starting with '.')
		if strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		// Skip known infrastructure directories
		if skipDirs[name] {
			return filepath.SkipDir
		}

		// Check if this directory contains SKILL.md
		skillMD := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillMD); err == nil {
			skills = append(skills, "./"+filepath.ToSlash(rel))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(skills)
	return skills, nil
}

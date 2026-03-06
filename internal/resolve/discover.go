package resolve

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/skill"
)

// DiscoverSkills finds skills in an in-memory file tree from a git repository.
// Given a list of all file paths (from ListTree) and a file reader function,
// it identifies directories containing SKILL.md and parses their frontmatter.
//
// Returns a list of skill names and a map of skill directory paths to their
// files (for integrity computation).
func DiscoverSkills(allPaths []string, readFile func(path string) ([]byte, error)) ([]DiscoveredSkill, error) {
	// Find all SKILL.md files
	var skillMDPaths []string
	for _, p := range allPaths {
		if filepath.Base(p) == "SKILL.md" {
			skillMDPaths = append(skillMDPaths, p)
		}
	}

	var skills []DiscoveredSkill
	for _, mdPath := range skillMDPaths {
		content, err := readFile(mdPath)
		if err != nil {
			continue
		}

		fm, err := skill.ParseFrontmatter(bytes.NewReader(content))
		if err != nil {
			continue
		}

		if fm.Name == "" {
			continue
		}

		// Skill directory is the parent of SKILL.md
		skillDir := filepath.Dir(mdPath)
		if skillDir == "." {
			skillDir = ""
		}

		// Collect all files in this skill directory
		var dirFiles []string
		prefix := skillDir
		if prefix != "" {
			prefix += "/"
		}
		for _, p := range allPaths {
			if prefix == "" || strings.HasPrefix(p, prefix) {
				// For root-level SKILL.md, include all files
				// For subdirectory SKILL.md, include only files in that dir tree
				if prefix == "" {
					dirFiles = append(dirFiles, p)
				} else {
					dirFiles = append(dirFiles, p)
				}
			}
		}

		skills = append(skills, DiscoveredSkill{
			Name:     fm.Name,
			Dir:      skillDir,
			Files:    dirFiles,
			MDPath:   mdPath,
		})
	}

	return skills, nil
}

// DiscoveredSkill represents a skill found during auto-discovery.
type DiscoveredSkill struct {
	// Name is the skill name from SKILL.md frontmatter.
	Name string

	// Dir is the skill directory path relative to repo root.
	Dir string

	// Files lists all file paths in this skill's directory.
	Files []string

	// MDPath is the path to the SKILL.md file.
	MDPath string
}

package resolve

import (
	"fmt"
	"testing"
)

func TestDiscoverSkillsBasic(t *testing.T) {
	allPaths := []string{
		"skills/lint/SKILL.md",
		"skills/lint/rules.yaml",
		"skills/review/SKILL.md",
		"README.md",
	}

	files := map[string][]byte{
		"skills/lint/SKILL.md":   []byte("---\nname: lint\ndescription: Lint skill.\n---\n"),
		"skills/review/SKILL.md": []byte("---\nname: review\ndescription: Review skill.\n---\n"),
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return content, nil
		}
		return nil, fmt.Errorf("not found")
	}

	skills, err := DiscoverSkills(allPaths, readFile)
	if err != nil {
		t.Fatalf("DiscoverSkills error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("Expected 2 skills, got %d", len(skills))
	}

	names := map[string]bool{}
	for _, s := range skills {
		names[s.Name] = true
	}
	if !names["lint"] || !names["review"] {
		t.Errorf("Expected lint and review skills, got %v", names)
	}
}

func TestDiscoverSkillsNoSkills(t *testing.T) {
	allPaths := []string{"README.md", "src/main.go"}

	readFile := func(path string) ([]byte, error) {
		return nil, fmt.Errorf("not found")
	}

	skills, err := DiscoverSkills(allPaths, readFile)
	if err != nil {
		t.Fatalf("DiscoverSkills error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Expected 0 skills, got %d", len(skills))
	}
}

func TestDiscoverSkillsInvalidFrontmatter(t *testing.T) {
	allPaths := []string{"SKILL.md"}

	files := map[string][]byte{
		"SKILL.md": []byte("no frontmatter here"),
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return content, nil
		}
		return nil, fmt.Errorf("not found")
	}

	skills, err := DiscoverSkills(allPaths, readFile)
	if err != nil {
		t.Fatalf("DiscoverSkills error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Expected 0 skills for invalid frontmatter, got %d", len(skills))
	}
}

func TestDiscoverSkillsMissingName(t *testing.T) {
	allPaths := []string{"skills/bad/SKILL.md"}

	files := map[string][]byte{
		"skills/bad/SKILL.md": []byte("---\ndescription: No name field.\n---\n"),
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return content, nil
		}
		return nil, fmt.Errorf("not found")
	}

	skills, err := DiscoverSkills(allPaths, readFile)
	if err != nil {
		t.Fatalf("DiscoverSkills error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Expected 0 skills for missing name, got %d", len(skills))
	}
}

package initcmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverSkills(t *testing.T) {
	// Create a temp directory structure
	root := t.TempDir()

	// Create skill directories with SKILL.md
	createSkillDir(t, root, "skills/lint-check")
	createSkillDir(t, root, "skills/review-pr")
	createSkillDir(t, root, "deep/nested/skill")

	// Create non-skill directories
	os.MkdirAll(filepath.Join(root, "src/utils"), 0o755)
	os.MkdirAll(filepath.Join(root, "docs"), 0o755)

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	expected := []string{
		"./deep/nested/skill",
		"./skills/lint-check",
		"./skills/review-pr",
	}

	if len(skills) != len(expected) {
		t.Fatalf("Expected %d skills, got %d: %v", len(expected), len(skills), skills)
	}

	sort.Strings(skills)
	for i, s := range expected {
		if skills[i] != s {
			t.Errorf("Skills[%d] = %q, want %q", i, skills[i], s)
		}
	}
}

func TestDiscoverSkillsSkipsHidden(t *testing.T) {
	root := t.TempDir()

	createSkillDir(t, root, ".hidden/skill")
	createSkillDir(t, root, "visible/skill")

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("Expected 1 skill, got %d: %v", len(skills), skills)
	}
	if skills[0] != "./visible/skill" {
		t.Errorf("Expected ./visible/skill, got %q", skills[0])
	}
}

func TestDiscoverSkillsSkipsInfra(t *testing.T) {
	root := t.TempDir()

	for _, dir := range []string{".git", "node_modules"} {
		createSkillDir(t, root, filepath.Join(dir, "skill"))
	}
	createSkillDir(t, root, "real-skill")

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("Expected 1 skill, got %d: %v", len(skills), skills)
	}
	if skills[0] != "./real-skill" {
		t.Errorf("Expected ./real-skill, got %q", skills[0])
	}
}

func TestDiscoverSkillsEmpty(t *testing.T) {
	root := t.TempDir()

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Expected 0 skills, got %d: %v", len(skills), skills)
	}
}

func TestDiscoverSkillsSortedOutput(t *testing.T) {
	root := t.TempDir()

	createSkillDir(t, root, "z-skill")
	createSkillDir(t, root, "a-skill")
	createSkillDir(t, root, "m-skill")

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	for i := 1; i < len(skills); i++ {
		if skills[i] < skills[i-1] {
			t.Errorf("Skills not sorted: %v", skills)
			break
		}
	}
}

func createSkillDir(t *testing.T, root, relPath string) {
	t.Helper()
	dir := filepath.Join(root, relPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("Creating dir %s: %v", dir, err)
	}
	skillMD := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("---\nname: test\n---\n"), 0o644); err != nil {
		t.Fatalf("Creating SKILL.md: %v", err)
	}
}

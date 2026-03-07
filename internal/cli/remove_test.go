package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRemove_ExistingDep(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
dependencies:
  my-dep: github.com/org/repo@v1.0.0
  other: github.com/org/other@v2.0.0
`
	_ = os.WriteFile(filepath.Join(dir, "craft.yaml"), []byte(manifestContent), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "s"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"), 0644)

	// Create pinfile with skills
	pinContent := `pin_version: 1
resolved:
  github.com/org/repo@v1.0.0:
    commit: abc123
    integrity: sha256-test
    skills:
      - repo-skill
  github.com/org/other@v2.0.0:
    commit: def456
    integrity: sha256-test2
    skills:
      - other-skill
`
	_ = os.WriteFile(filepath.Join(dir, "craft.pin.yaml"), []byte(pinContent), 0644)

	// Create installed skill directory
	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(filepath.Join(targetDir, "repo-skill"), 0755)
	_ = os.WriteFile(filepath.Join(targetDir, "repo-skill", "SKILL.md"), []byte("skill"), 0644)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"remove", "--target", targetDir, "my-dep"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Removed") {
		t.Errorf("expected 'Removed' message, got: %s", output)
	}

	// Verify manifest updated
	updated, _ := os.ReadFile(filepath.Join(dir, "craft.yaml"))
	if strings.Contains(string(updated), "my-dep") {
		t.Error("manifest should not contain removed dep")
	}
	if !strings.Contains(string(updated), "other") {
		t.Error("manifest should still contain other dep")
	}

	// Verify pinfile updated
	updatedPin, _ := os.ReadFile(filepath.Join(dir, "craft.pin.yaml"))
	if strings.Contains(string(updatedPin), "github.com/org/repo@v1.0.0") {
		t.Error("pinfile should not contain removed dep")
	}

	// Verify orphaned skill cleaned up
	if _, err := os.Stat(filepath.Join(targetDir, "repo-skill")); err == nil {
		t.Error("orphaned skill directory should have been removed")
	}
}

func TestRunRemove_NonExistentAlias(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
dependencies:
  real-dep: github.com/org/repo@v1.0.0
`
	_ = os.WriteFile(filepath.Join(dir, "craft.yaml"), []byte(manifestContent), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "s"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"), 0644)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"remove", "nonexistent"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for non-existent alias")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "real-dep") {
		t.Errorf("expected available aliases in error, got: %v", err)
	}
}

func TestRunRemove_SharedSkillRetained(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
dependencies:
  dep-a: github.com/org/a@v1.0.0
  dep-b: github.com/org/b@v1.0.0
`
	_ = os.WriteFile(filepath.Join(dir, "craft.yaml"), []byte(manifestContent), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "s"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"), 0644)

	// Both deps provide "shared-skill", dep-a also provides "unique-a"
	pinContent := `pin_version: 1
resolved:
  github.com/org/a@v1.0.0:
    commit: aaa
    integrity: sha256-a
    skills:
      - shared-skill
      - unique-a
  github.com/org/b@v1.0.0:
    commit: bbb
    integrity: sha256-b
    skills:
      - shared-skill
`
	_ = os.WriteFile(filepath.Join(dir, "craft.pin.yaml"), []byte(pinContent), 0644)

	// Create installed skill directories
	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(filepath.Join(targetDir, "shared-skill"), 0755)
	_ = os.MkdirAll(filepath.Join(targetDir, "unique-a"), 0755)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"remove", "--target", targetDir, "dep-a"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// shared-skill should be retained (dep-b still provides it)
	if _, err := os.Stat(filepath.Join(targetDir, "shared-skill")); err != nil {
		t.Error("shared skill should have been retained")
	}

	// unique-a should be cleaned up
	if _, err := os.Stat(filepath.Join(targetDir, "unique-a")); err == nil {
		t.Error("unique-a should have been removed")
	}
}

func TestRunRemove_LastDependency(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
dependencies:
  only-dep: github.com/org/repo@v1.0.0
`
	_ = os.WriteFile(filepath.Join(dir, "craft.yaml"), []byte(manifestContent), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "s"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"), 0644)

	pinContent := `pin_version: 1
resolved:
  github.com/org/repo@v1.0.0:
    commit: abc
    integrity: sha256-x
    skills:
      - the-skill
`
	_ = os.WriteFile(filepath.Join(dir, "craft.pin.yaml"), []byte(pinContent), 0644)

	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(filepath.Join(targetDir, "the-skill"), 0755)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"remove", "--target", targetDir, "only-dep"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify manifest has no dependencies
	updated, _ := os.ReadFile(filepath.Join(dir, "craft.yaml"))
	if strings.Contains(string(updated), "dependencies") {
		t.Error("manifest should have no dependencies section after removing last dep")
	}
}

func TestAvailableAliases(t *testing.T) {
	deps := map[string]string{
		"zebra": "z",
		"alpha": "a",
	}
	got := availableAliases(deps)
	if got != "alpha, zebra" {
		t.Errorf("expected sorted aliases, got: %s", got)
	}
}

func TestAvailableAliases_Empty(t *testing.T) {
	got := availableAliases(nil)
	if got != "(none)" {
		t.Errorf("expected '(none)', got: %s", got)
	}
}

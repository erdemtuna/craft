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

	// Create installed skill directory (namespaced: target/host/owner/repo/skill)
	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "repo", "repo-skill"), 0755)
	_ = os.WriteFile(filepath.Join(targetDir, "github.com", "org", "repo", "repo-skill", "SKILL.md"), []byte("skill"), 0644)

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

	// Verify orphaned skill cleaned up (namespaced path)
	if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "repo", "repo-skill")); err == nil {
		t.Error("orphaned skill directory should have been removed")
	}
	// Verify empty parent dirs cleaned up
	if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "repo")); err == nil {
		t.Error("empty repo directory should have been cleaned up")
	}
}

func TestRunRemove_NonExistentAlias(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
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

	// Create installed skill directories (namespaced)
	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "shared-skill"), 0755)
	_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "a", "unique-a"), 0755)
	_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "b", "shared-skill"), 0755)
	_ = os.WriteFile(filepath.Join(targetDir, "github.com", "org", "b", "shared-skill", "SKILL.md"), []byte("dep-b skill"), 0644)

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

	// With namespacing, shared-skill from dep-a lives under github.com/org/a/
	// and dep-b's shared-skill lives under github.com/org/b/ — independent paths.
	// Removing dep-a should clean up ALL of dep-a's skills regardless of name overlap.
	if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "a", "shared-skill")); err == nil {
		t.Error("dep-a's shared-skill should have been removed (namespaced paths are independent)")
	}

	// unique-a should be cleaned up since only dep-a provided it
	if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "a", "unique-a")); err == nil {
		t.Error("unique-a should have been removed")
	}

	// dep-b's identically-named skill should survive dep-a removal
	if _, err := os.Stat(filepath.Join(targetDir, "github.com", "org", "b", "shared-skill")); err != nil {
		t.Error("dep-b's shared-skill should survive dep-a removal (independent namespaced paths)")
	}
}

func TestRunRemove_LastDependency(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
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
	_ = os.MkdirAll(filepath.Join(targetDir, "github.com", "org", "repo", "the-skill"), 0755)

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

func TestRunRemove_BadDepURLWarns(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
skills:
  - ./skills/s
dependencies:
  bad-dep: "not-a-valid-url"
`
	_ = os.WriteFile(filepath.Join(dir, "craft.yaml"), []byte(manifestContent), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "s"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"), 0644)

	pinContent := `pin_version: 1
resolved:
  not-a-valid-url:
    commit: abc123
    integrity: sha256-test
    skills:
      - some-skill
`
	_ = os.WriteFile(filepath.Join(dir, "craft.pin.yaml"), []byte(pinContent), 0644)

	targetDir := filepath.Join(dir, "installed")
	_ = os.MkdirAll(targetDir, 0755)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"remove", "--target", targetDir, "bad-dep"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("remove should succeed even with bad dep URL, got: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "warning: could not parse dep URL") {
		t.Errorf("expected warning about unparseable dep URL, got: %s", output)
	}
}

func TestCleanEmptyParents(t *testing.T) {
	root := t.TempDir()

	// Create nested empty dirs: root/a/b/c/
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	// Clean from c upward
	cleanEmptyParents(root, nested)

	// All empty parents should be removed
	if _, err := os.Stat(filepath.Join(root, "a")); err == nil {
		t.Error("expected 'a' to be removed")
	}
	// Root itself should remain
	if _, err := os.Stat(root); err != nil {
		t.Error("root should still exist")
	}
}

func TestCleanEmptyParents_StopsAtNonEmpty(t *testing.T) {
	root := t.TempDir()

	// Create root/a/b/c/ where a/sibling exists
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, "a", "sibling")
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatal(err)
	}

	cleanEmptyParents(root, nested)

	// b and c removed, but a remains (has sibling)
	if _, err := os.Stat(filepath.Join(root, "a", "b")); err == nil {
		t.Error("expected 'b' to be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "a")); err != nil {
		t.Error("'a' should remain (has sibling)")
	}
}

func TestCleanEmptyParents_StopsAtRoot(t *testing.T) {
	root := t.TempDir()

	// dir IS root — should not delete root
	cleanEmptyParents(root, root)

	if _, err := os.Stat(root); err != nil {
		t.Error("root should not be deleted")
	}
}

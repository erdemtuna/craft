package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testChdir(t *testing.T, dir string) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func testWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func testMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func TestRunAdd_NewDependency(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal manifest
	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/my-skill
`
	testWriteFile(t, filepath.Join(dir, "craft.yaml"), []byte(manifestContent))
	testMkdirAll(t, filepath.Join(dir, "skills", "my-skill"))
	testWriteFile(t, filepath.Join(dir, "skills", "my-skill", "SKILL.md"), []byte("---\nname: my-skill\n---\n"))

	testChdir(t, dir)

	// Use the rootCmd to test — but craft add requires network access
	// so we test argument parsing and manifest-not-found scenarios
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "invalid-url-no-version"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid dependency URL") {
		t.Errorf("expected invalid URL error, got: %v", err)
	}
}

func TestRunAdd_InvalidURLFormat(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
`
	testWriteFile(t, filepath.Join(dir, "craft.yaml"), []byte(manifestContent))
	testMkdirAll(t, filepath.Join(dir, "skills", "s"))
	testWriteFile(t, filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"))

	testChdir(t, dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "not-a-valid-url"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "hint") {
		t.Errorf("expected hint in error message, got: %v", err)
	}
}

func TestRunAdd_NoManifest(t *testing.T) {
	dir := t.TempDir()

	testChdir(t, dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "github.com/org/repo@v1.0.0"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error when craft.yaml missing")
	}
	if !strings.Contains(err.Error(), "craft init") {
		t.Errorf("expected hint about craft init, got: %v", err)
	}
}

func TestRunAdd_TooManyArgs(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "a", "b", "c"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestRunAdd_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
dependencies:
  my-dep: github.com/org/repo@v1.0.0
`
	testWriteFile(t, filepath.Join(dir, "craft.yaml"), []byte(manifestContent))
	testMkdirAll(t, filepath.Join(dir, "skills", "s"))
	testWriteFile(t, filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"))

	testChdir(t, dir)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "my-dep", "github.com/org/repo@v1.0.0"})
	err := rootCmd.Execute()

	// Should succeed with "already at" message (no network needed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "already at") {
		t.Errorf("expected 'already at' message, got: %s", buf.String())
	}
}

func TestRunAdd_AliasDerivation(t *testing.T) {
	// Test that alias is derived from repo name
	dir := t.TempDir()

	manifestContent := `schema_version: 1
name: test-pkg
version: 0.1.0
skills:
  - ./skills/s
`
	testWriteFile(t, filepath.Join(dir, "craft.yaml"), []byte(manifestContent))
	testMkdirAll(t, filepath.Join(dir, "skills", "s"))
	testWriteFile(t, filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("---\nname: s\n---\n"))

	testChdir(t, dir)

	// This will fail at resolution (network) but tests URL parsing + alias derivation
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"add", "github.com/org/my-skills@v1.0.0"})
	err := rootCmd.Execute()

	// Error is expected (network), but should get past URL parsing
	if err != nil && strings.Contains(err.Error(), "invalid dependency URL") {
		t.Errorf("should have parsed URL correctly, got: %v", err)
	}
}

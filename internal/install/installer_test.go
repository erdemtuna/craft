package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCreatesStructure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"my-skill": {
			"SKILL.md":  []byte("---\nname: my-skill\n---\n"),
			"rules.txt": []byte("some rules"),
		},
	}

	if err := Install(target, skills); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(target, "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(content) != "---\nname: my-skill\n---\n" {
		t.Errorf("Unexpected content: %q", content)
	}
}

func TestInstallOverwrites(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")

	skills1 := map[string]map[string][]byte{
		"my-skill": {"SKILL.md": []byte("old")},
	}
	Install(target, skills1)

	skills2 := map[string]map[string][]byte{
		"my-skill": {"SKILL.md": []byte("new")},
	}
	if err := Install(target, skills2); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(target, "my-skill", "SKILL.md"))
	if string(content) != "new" {
		t.Errorf("Expected overwritten content, got %q", content)
	}
}

func TestInstallEmpty(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	if err := Install(target, map[string]map[string][]byte{}); err != nil {
		t.Fatalf("Install error: %v", err)
	}
}

func TestInstallRejectsTraversalSkillName(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"../../etc/malicious": {
			"SKILL.md": []byte("pwned"),
		},
	}
	err := Install(target, skills)
	if err == nil {
		t.Fatal("expected error for path-traversal skill name, got nil")
	}
	if !strings.Contains(err.Error(), "escapes target directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInstallRejectsTraversalFilePath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"legit-skill": {
			"../../etc/passwd": []byte("pwned"),
		},
	}
	err := Install(target, skills)
	if err == nil {
		t.Fatal("expected error for path-traversal file path, got nil")
	}
	if !strings.Contains(err.Error(), "escapes skill directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInstallAllowsNormalSkillNames(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"my-skill": {
			"SKILL.md":          []byte("ok"),
			"subdir/rules.txt":  []byte("nested ok"),
		},
	}
	if err := Install(target, skills); err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "my-skill", "subdir", "rules.txt")); err != nil {
		t.Fatalf("expected nested file to exist: %v", err)
	}
}

func TestInstallRejectsDotSkillName(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		".": {
			"SKILL.md": []byte("sneaky"),
		},
	}
	err := Install(target, skills)
	if err == nil {
		t.Fatal("expected error for '.' skill name, got nil")
	}
}

func TestInstallRejectsEmptySkillName(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"": {
			"SKILL.md": []byte("sneaky"),
		},
	}
	err := Install(target, skills)
	if err == nil {
		t.Fatal("expected error for empty skill name, got nil")
	}
}

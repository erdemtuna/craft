package install

import (
	"os"
	"path/filepath"
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

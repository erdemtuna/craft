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
	if err := Install(target, skills1); err != nil {
		t.Fatalf("first Install error: %v", err)
	}

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
			"SKILL.md":         []byte("ok"),
			"subdir/rules.txt": []byte("nested ok"),
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

func TestInstallCleansUpStagingOnError(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	// Install a skill with a path-traversal file (will fail validation)
	skills := map[string]map[string][]byte{
		"bad-skill": {
			"../../etc/passwd": []byte("pwned"),
		},
	}
	_ = Install(target, skills)
	entries, _ := os.ReadDir(target)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".staging") {
			t.Errorf("staging directory %q was not cleaned up", e.Name())
		}
	}
}

func TestInstallAtomicOverwrite(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	// First install
	skills1 := map[string]map[string][]byte{
		"my-skill": {"SKILL.md": []byte("v1")},
	}
	if err := Install(target, skills1); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Second install overwrites atomically
	skills2 := map[string]map[string][]byte{
		"my-skill": {"SKILL.md": []byte("v2"), "extra.txt": []byte("new file")},
	}
	if err := Install(target, skills2); err != nil {
		t.Fatalf("second install: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(target, "my-skill", "SKILL.md"))
	if string(content) != "v2" {
		t.Errorf("expected v2, got %q", content)
	}
	// Old files should be gone
	if _, err := os.ReadDir(filepath.Join(target, "my-skill")); err != nil {
		t.Fatalf("skill dir should exist: %v", err)
	}
}

func TestInstallCompositeKeys(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"github.com/org/repo/my-skill": {
			"SKILL.md": []byte("---\nname: my-skill\n---\n"),
		},
		"github.com/other/tools/my-skill": {
			"SKILL.md": []byte("---\nname: my-skill\n---\nfrom other"),
		},
	}

	if err := Install(target, skills); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	// Both skills should exist at different paths
	content1, err := os.ReadFile(filepath.Join(target, "github.com", "org", "repo", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile error for org/repo: %v", err)
	}
	if !strings.Contains(string(content1), "name: my-skill") {
		t.Errorf("Unexpected content for org/repo: %q", content1)
	}

	content2, err := os.ReadFile(filepath.Join(target, "github.com", "other", "tools", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile error for other/tools: %v", err)
	}
	if !strings.Contains(string(content2), "from other") {
		t.Errorf("Unexpected content for other/tools: %q", content2)
	}
}

func TestFlatKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard composite key", "github.com/org/repo/my-skill", "github.com--org--repo--my-skill"},
		{"real PAW key", "github.com/lossyrob/phased-agent-workflow/paw-implement", "github.com--lossyrob--phased-agent-workflow--paw-implement"},
		{"anthropic skills", "github.com/anthropics/skills/skill-creator", "github.com--anthropics--skills--skill-creator"},
		{"simple skill name", "simple-skill", "simple-skill"},
		{"custom host with dots", "host.name/owner/repo/skill", "host.name--owner--repo--skill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlatKey(tt.input)
			if got != tt.want {
				t.Errorf("FlatKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFlatKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"dots in org", "github.com/my.org/repo/skill", "github.com--my.org--repo--skill"},
		{"dots in repo", "github.com/org/my.repo/skill", "github.com--org--my.repo--skill"},
		{"dots in skill", "github.com/org/repo/my.skill", "github.com--org--repo--my.skill"},
		{"dots everywhere", "git.hub.com/my.org/my.repo/my.skill", "git.hub.com--my.org--my.repo--my.skill"},
		{"mixed casing", "GitHub.com/MyOrg/Repo/Skill", "GitHub.com--MyOrg--Repo--Skill"},
		{"many components", "a/b/c/d/e", "a--b--c--d--e"},
		{"no slashes no dots", "plain-name", "plain-name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlatKey(tt.input)
			if got != tt.want {
				t.Errorf("FlatKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInstallFlatCreatesStructure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"github.com/org/repo/my-skill": {
			"SKILL.md": []byte("flat skill"),
		},
	}

	if err := InstallFlat(target, skills); err != nil {
		t.Fatalf("InstallFlat error: %v", err)
	}

	// Should be flat, not nested
	flatDir := filepath.Join(target, "github.com--org--repo--my-skill")
	content, err := os.ReadFile(filepath.Join(flatDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(content) != "flat skill" {
		t.Errorf("Unexpected content: %q", content)
	}

	// Nested path should NOT exist
	nestedDir := filepath.Join(target, "github.com", "org", "repo", "my-skill")
	if _, err := os.Stat(nestedDir); err == nil {
		t.Error("nested directory should not exist after InstallFlat")
	}
}

func TestInstallFlatMultiPackage(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"github.com/org/repo/skill-a": {
			"SKILL.md": []byte("a"),
		},
		"github.com/other/tools/skill-b": {
			"SKILL.md": []byte("b"),
		},
	}

	if err := InstallFlat(target, skills); err != nil {
		t.Fatalf("InstallFlat error: %v", err)
	}

	// Both should be direct children of target
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names["github.com--org--repo--skill-a"] {
		t.Error("missing github.com--org--repo--skill-a")
	}
	if !names["github.com--other--tools--skill-b"] {
		t.Error("missing github.com--other--tools--skill-b")
	}
}

func TestInstallFlatSameName(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"github.com/org/repo/my-skill": {
			"SKILL.md": []byte("from org"),
		},
		"github.com/other/tools/my-skill": {
			"SKILL.md": []byte("from other"),
		},
	}

	if err := InstallFlat(target, skills); err != nil {
		t.Fatalf("InstallFlat error: %v", err)
	}

	// Same leaf name, different flat keys — no collision
	c1, err := os.ReadFile(filepath.Join(target, "github.com--org--repo--my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile org: %v", err)
	}
	if string(c1) != "from org" {
		t.Errorf("org content: %q", c1)
	}

	c2, err := os.ReadFile(filepath.Join(target, "github.com--other--tools--my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile other: %v", err)
	}
	if string(c2) != "from other" {
		t.Errorf("other content: %q", c2)
	}
}

func TestInstallFlatOverwrites(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")

	skills1 := map[string]map[string][]byte{
		"github.com/org/repo/my-skill": {"SKILL.md": []byte("v1")},
	}
	if err := InstallFlat(target, skills1); err != nil {
		t.Fatalf("first InstallFlat: %v", err)
	}

	skills2 := map[string]map[string][]byte{
		"github.com/org/repo/my-skill": {"SKILL.md": []byte("v2")},
	}
	if err := InstallFlat(target, skills2); err != nil {
		t.Fatalf("second InstallFlat: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(target, "github.com--org--repo--my-skill", "SKILL.md"))
	if string(content) != "v2" {
		t.Errorf("expected v2, got %q", content)
	}
}

func TestInstallFlatDistinguishesDotFromDash(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	// Dots and hyphens are distinct — no collision with injective encoding
	skills := map[string]map[string][]byte{
		"github.com/org/my.repo/skill": {"SKILL.md": []byte("dot")},
		"github.com/org/my-repo/skill": {"SKILL.md": []byte("dash")},
	}

	if err := InstallFlat(target, skills); err != nil {
		t.Fatalf("InstallFlat error: %v", err)
	}

	c1, _ := os.ReadFile(filepath.Join(target, "github.com--org--my.repo--skill", "SKILL.md"))
	if string(c1) != "dot" {
		t.Errorf("dot repo content: %q", c1)
	}
	c2, _ := os.ReadFile(filepath.Join(target, "github.com--org--my-repo--skill", "SKILL.md"))
	if string(c2) != "dash" {
		t.Errorf("dash repo content: %q", c2)
	}
}

func TestInstallFlatRejectsEmptyKey(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skills")
	skills := map[string]map[string][]byte{
		"": {"SKILL.md": []byte("bad")},
	}
	err := InstallFlat(target, skills)
	if err == nil {
		t.Fatal("expected error for empty composite key")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

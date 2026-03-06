package pinfile

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	input := `pin_version: 1
resolved:
  github.com/example/git-skills@v1.0.0:
    commit: a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
    integrity: sha256-Xk9jR2mN5pQ8vW3yB7cF1dA4hL6tS0uE9iO2wR5nM3s=
    skills:
      - git-commit
      - git-branch
`
	p, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if p.PinVersion != 1 {
		t.Errorf("PinVersion = %d, want 1", p.PinVersion)
	}

	entry, ok := p.Resolved["github.com/example/git-skills@v1.0.0"]
	if !ok {
		t.Fatal("Expected resolved entry for github.com/example/git-skills@v1.0.0")
	}
	if entry.Commit != "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2" {
		t.Errorf("Commit = %q", entry.Commit)
	}
	if entry.Integrity != "sha256-Xk9jR2mN5pQ8vW3yB7cF1dA4hL6tS0uE9iO2wR5nM3s=" {
		t.Errorf("Integrity = %q", entry.Integrity)
	}
	if len(entry.Skills) != 2 {
		t.Fatalf("Skills length = %d, want 2", len(entry.Skills))
	}
}

func TestParseMissingFields(t *testing.T) {
	input := `pin_version: 1
resolved:
  github.com/example/repo@v1.0.0:
    commit: abc123
`
	p, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should parse but have empty integrity and nil skills
	entry := p.Resolved["github.com/example/repo@v1.0.0"]
	if entry.Integrity != "" {
		t.Errorf("Expected empty integrity, got %q", entry.Integrity)
	}
	if entry.Skills != nil {
		t.Errorf("Expected nil skills, got %v", entry.Skills)
	}
}

func TestParseMalformedYAML(t *testing.T) {
	input := `pin_version: [invalid
resolved: {broken`

	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("Parse should fail on malformed YAML")
	}
}

func TestParseFile(t *testing.T) {
	p, err := ParseFile("../../testdata/pinfiles/valid.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if p.PinVersion != 1 {
		t.Errorf("PinVersion = %d, want 1", p.PinVersion)
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := ParseFile("nonexistent.yaml")
	if err == nil {
		t.Fatal("ParseFile should fail on missing file")
	}
}

package manifest

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	input := `schema_version: 1
name: code-quality
description: Test package.
license: MIT
skills:
  - ./skills/lint
  - ./skills/review
dependencies:
  git-ops: github.com/example/git@v1.0.0
metadata:
  author: tester
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if m.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", m.SchemaVersion)
	}
	if m.Name != "code-quality" {
		t.Errorf("Name = %q, want %q", m.Name, "code-quality")
	}
	if m.Description != "Test package." {
		t.Errorf("Description = %q, want %q", m.Description, "Test package.")
	}
	if m.License != "MIT" {
		t.Errorf("License = %q, want %q", m.License, "MIT")
	}
	if len(m.Skills) != 2 {
		t.Fatalf("Skills length = %d, want 2", len(m.Skills))
	}
	if m.Skills[0] != "./skills/lint" {
		t.Errorf("Skills[0] = %q, want %q", m.Skills[0], "./skills/lint")
	}
	if len(m.Dependencies) != 1 {
		t.Fatalf("Dependencies length = %d, want 1", len(m.Dependencies))
	}
	if m.Dependencies["git-ops"] != "github.com/example/git@v1.0.0" {
		t.Errorf("Dependencies[git-ops] = %q", m.Dependencies["git-ops"])
	}
	if m.Metadata["author"] != "tester" {
		t.Errorf("Metadata[author] = %q", m.Metadata["author"])
	}
}

func TestParseMinimal(t *testing.T) {
	input := `schema_version: 1
name: minimal
skills:
  - ./my-skill
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if m.Description != "" {
		t.Errorf("Description should be empty, got %q", m.Description)
	}
	if m.License != "" {
		t.Errorf("License should be empty, got %q", m.License)
	}
	if len(m.Dependencies) != 0 {
		t.Errorf("Dependencies should be empty, got %v", m.Dependencies)
	}
}

func TestParseUnknownFields(t *testing.T) {
	input := `schema_version: 1
name: extras
skills:
  - ./skill
custom_field: hello
another: 42
`
	m, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Unknown fields should not cause errors (forward compatibility).
	if m.Name != "extras" {
		t.Errorf("Name = %q, want %q", m.Name, "extras")
	}
}

func TestParseMalformedYAML(t *testing.T) {
	input := `schema_version: [invalid
name: {broken`

	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("Parse should fail on malformed YAML")
	}
	if !strings.Contains(err.Error(), "parsing manifest YAML") {
		t.Errorf("Error should mention YAML parsing, got: %v", err)
	}
}

func TestParseFile(t *testing.T) {
	m, err := ParseFile("../../testdata/manifests/valid.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if m.Name != "code-quality" {
		t.Errorf("Name = %q, want %q", m.Name, "code-quality")
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := ParseFile("nonexistent.yaml")
	if err == nil {
		t.Fatal("ParseFile should fail on missing file")
	}
}

package skill

import (
	"strings"
	"testing"
)

func TestParseFrontmatterValid(t *testing.T) {
	input := `---
name: lint-check
description: Automated linting and code style checking.
---

# Lint Check

This skill provides automated linting.
`
	fm, err := ParseFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseFrontmatter failed: %v", err)
	}

	if fm.Name != "lint-check" {
		t.Errorf("Name = %q, want %q", fm.Name, "lint-check")
	}
	if fm.Description != "Automated linting and code style checking." {
		t.Errorf("Description = %q", fm.Description)
	}
	if len(fm.Extra) != 0 {
		t.Errorf("Extra should be empty, got %v", fm.Extra)
	}
}

func TestParseFrontmatterWithExtras(t *testing.T) {
	input := `---
name: my-skill
description: A test skill.
custom_field: extra-value
tags:
  - testing
  - example
---

# My Skill
`
	fm, err := ParseFrontmatter(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseFrontmatter failed: %v", err)
	}

	if fm.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "my-skill")
	}

	if fm.Extra == nil {
		t.Fatal("Extra should not be nil")
	}
	if _, ok := fm.Extra["custom_field"]; !ok {
		t.Error("Extra should contain 'custom_field'")
	}
	if _, ok := fm.Extra["tags"]; !ok {
		t.Error("Extra should contain 'tags'")
	}
}

func TestParseFrontmatterNoDelimiters(t *testing.T) {
	input := `# No Frontmatter

This SKILL.md has no YAML frontmatter delimiters.
`
	_, err := ParseFrontmatter(strings.NewReader(input))
	if err == nil {
		t.Fatal("Should fail when no frontmatter delimiters present")
	}
	if !strings.Contains(err.Error(), "does not start with '---'") {
		t.Errorf("Error should mention missing delimiter, got: %v", err)
	}
}

func TestParseFrontmatterMissingClosing(t *testing.T) {
	input := `---
name: unclosed
description: No closing delimiter.
`
	_, err := ParseFrontmatter(strings.NewReader(input))
	if err == nil {
		t.Fatal("Should fail when closing delimiter is missing")
	}
	if !strings.Contains(err.Error(), "closing '---'") {
		t.Errorf("Error should mention missing closing delimiter, got: %v", err)
	}
}

func TestParseFrontmatterEmpty(t *testing.T) {
	input := `---
---

# Empty frontmatter
`
	_, err := ParseFrontmatter(strings.NewReader(input))
	if err == nil {
		t.Fatal("Should fail when frontmatter content is empty")
	}
	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("Error should mention empty content, got: %v", err)
	}
}

func TestParseFrontmatterMalformedYAML(t *testing.T) {
	input := `---
name: [invalid yaml
this is not valid: {
---

# Malformed
`
	_, err := ParseFrontmatter(strings.NewReader(input))
	if err == nil {
		t.Fatal("Should fail on malformed YAML in frontmatter")
	}
	if !strings.Contains(err.Error(), "parsing frontmatter") {
		t.Errorf("Error should mention YAML parsing, got: %v", err)
	}
}

func TestParseFrontmatterEmptyFile(t *testing.T) {
	_, err := ParseFrontmatter(strings.NewReader(""))
	if err == nil {
		t.Fatal("Should fail on empty file")
	}
}

func TestParseFrontmatterFile(t *testing.T) {
	fm, err := ParseFrontmatterFile("../../testdata/skills/valid.md")
	if err != nil {
		t.Fatalf("ParseFrontmatterFile failed: %v", err)
	}
	if fm.Name != "lint-check" {
		t.Errorf("Name = %q, want %q", fm.Name, "lint-check")
	}
}

func TestParseFrontmatterFileNotFound(t *testing.T) {
	_, err := ParseFrontmatterFile("nonexistent.md")
	if err == nil {
		t.Fatal("ParseFrontmatterFile should fail on missing file")
	}
}

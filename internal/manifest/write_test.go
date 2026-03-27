package manifest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestWriteRoundTrip(t *testing.T) {
	original := &Manifest{
		SchemaVersion: 1,
		Name:          "round-trip",
		Description:   "Test round-trip.",
		License:       "MIT",
		Skills:        []string{"./skills/one", "./skills/two"},
		Dependencies: map[string]DependencySpec{
			"dep-a": {URL: "github.com/org/repo-a@v1.0.0"},
		},
		Metadata: map[string]string{
			"author": "test",
		},
	}

	var buf bytes.Buffer
	if err := Write(original, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.SchemaVersion != original.SchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d", parsed.SchemaVersion, original.SchemaVersion)
	}
	if parsed.Name != original.Name {
		t.Errorf("Name: got %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Description != original.Description {
		t.Errorf("Description: got %q, want %q", parsed.Description, original.Description)
	}
	if parsed.License != original.License {
		t.Errorf("License: got %q, want %q", parsed.License, original.License)
	}
	if len(parsed.Skills) != len(original.Skills) {
		t.Fatalf("Skills: got %d items, want %d", len(parsed.Skills), len(original.Skills))
	}
	for i, s := range original.Skills {
		if parsed.Skills[i] != s {
			t.Errorf("Skills[%d]: got %q, want %q", i, parsed.Skills[i], s)
		}
	}
	if parsed.Dependencies["dep-a"].URL != original.Dependencies["dep-a"].URL {
		t.Errorf("Dependencies[dep-a]: got %q, want %q", parsed.Dependencies["dep-a"].URL, original.Dependencies["dep-a"].URL)
	}
	if parsed.Metadata["author"] != original.Metadata["author"] {
		t.Errorf("Metadata[author]: got %q, want %q", parsed.Metadata["author"], original.Metadata["author"])
	}
}

func TestWriteFieldOrder(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "ordered",
		Description:   "Test ordering.",
		Skills:        []string{"./skill"},
	}

	var buf bytes.Buffer
	if err := Write(m, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// schema_version should appear before name, which should appear before skills
	schemaIdx := strings.Index(output, "schema_version")
	nameIdx := strings.Index(output, "\nname:")
	skillsIdx := strings.Index(output, "\nskills:")

	if schemaIdx == -1 || nameIdx == -1 || skillsIdx == -1 {
		t.Fatalf("Missing expected fields in output:\n%s", output)
	}

	if schemaIdx >= nameIdx {
		t.Error("schema_version should appear before name")
	}
	if nameIdx >= skillsIdx {
		t.Error("name should appear before skills")
	}
}

func TestWriteOmitsEmpty(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "minimal",
		Skills:        []string{"./skill"},
	}

	var buf bytes.Buffer
	if err := Write(m, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "description") {
		t.Error("Empty description should be omitted")
	}
	if strings.Contains(output, "license") {
		t.Error("Empty license should be omitted")
	}
	if strings.Contains(output, "dependencies") {
		t.Error("Empty dependencies should be omitted")
	}
	if strings.Contains(output, "metadata") {
		t.Error("Empty metadata should be omitted")
	}
}

func TestWriteMapKeyOrder(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "key-order",
		Skills:        []string{"./skill"},
		Dependencies: map[string]DependencySpec{
			"charlie": {URL: "github.com/org/charlie@v1.0.0"},
			"alpha":   {URL: "github.com/org/alpha@v1.0.0"},
			"bravo":   {URL: "github.com/org/bravo@v1.0.0"},
		},
		Metadata: map[string]string{
			"zebra": "z",
			"apple": "a",
			"mango": "m",
		},
	}

	// Write multiple times and verify output is identical (deterministic)
	var first string
	for i := 0; i < 5; i++ {
		var buf bytes.Buffer
		if err := Write(m, &buf); err != nil {
			t.Fatalf("Write failed on iteration %d: %v", i, err)
		}
		if i == 0 {
			first = buf.String()
		} else if buf.String() != first {
			t.Fatalf("Non-deterministic output on iteration %d:\nfirst:\n%s\ngot:\n%s", i, first, buf.String())
		}
	}

	// Verify keys are sorted alphabetically
	alphaIdx := strings.Index(first, "alpha")
	bravoIdx := strings.Index(first, "bravo")
	charlieIdx := strings.Index(first, "charlie")
	if alphaIdx >= bravoIdx || bravoIdx >= charlieIdx {
		t.Errorf("Dependencies keys not sorted: alpha@%d, bravo@%d, charlie@%d", alphaIdx, bravoIdx, charlieIdx)
	}

	appleIdx := strings.Index(first, "apple")
	mangoIdx := strings.Index(first, "mango")
	zebraIdx := strings.Index(first, "zebra")
	if appleIdx >= mangoIdx || mangoIdx >= zebraIdx {
		t.Errorf("Metadata keys not sorted: apple@%d, mango@%d, zebra@%d", appleIdx, mangoIdx, zebraIdx)
	}
}

// errWriter is an io.Writer that always returns an error.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("simulated write failure")
}

func TestWriteRoundTripStructuredDeps(t *testing.T) {
	original := &Manifest{
		SchemaVersion: 1,
		Name:          "structured-round-trip",
		Skills:        []string{"./skills/one"},
		Dependencies: map[string]DependencySpec{
			"acme": {
				URL:    "github.com/acme/skills@v1.0.0",
				Select: []string{"skills/docx", "skills/pdf"},
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(original, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "url:") {
		t.Error("Structured dep should contain 'url:' key")
	}
	if !strings.Contains(output, "select:") {
		t.Error("Structured dep should contain 'select:' key")
	}

	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	dep := parsed.Dependencies["acme"]
	if dep.URL != "github.com/acme/skills@v1.0.0" {
		t.Errorf("URL: got %q, want %q", dep.URL, "github.com/acme/skills@v1.0.0")
	}
	if len(dep.Select) != 2 {
		t.Fatalf("Select length: got %d, want 2", len(dep.Select))
	}
	if dep.Select[0] != "skills/docx" || dep.Select[1] != "skills/pdf" {
		t.Errorf("Select: got %v, want [skills/docx skills/pdf]", dep.Select)
	}
}

func TestWriteRoundTripMixedDeps(t *testing.T) {
	original := &Manifest{
		SchemaVersion: 1,
		Name:          "mixed-round-trip",
		Skills:        []string{"./skills/one"},
		Dependencies: map[string]DependencySpec{
			"simple": {URL: "github.com/org/simple@v1.0.0"},
			"structured": {
				URL:    "github.com/org/structured@v2.0.0",
				Select: []string{"skills/a"},
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(original, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()
	// Simple dep should be a scalar value
	if !strings.Contains(output, "simple: github.com/org/simple@v1.0.0") {
		t.Errorf("Simple dep should be scalar in output:\n%s", output)
	}

	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Dependencies["simple"].URL != "github.com/org/simple@v1.0.0" {
		t.Errorf("Simple dep URL mismatch")
	}
	if len(parsed.Dependencies["simple"].Select) != 0 {
		t.Errorf("Simple dep should have no Select")
	}
	if parsed.Dependencies["structured"].URL != "github.com/org/structured@v2.0.0" {
		t.Errorf("Structured dep URL mismatch")
	}
	if len(parsed.Dependencies["structured"].Select) != 1 {
		t.Errorf("Structured dep should have 1 Select entry")
	}
}

func TestWriteError(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "err-test",
		Skills:        []string{"./skill"},
	}

	err := Write(m, errWriter{})
	if err == nil {
		t.Fatal("Expected error from Write with failing writer")
	}
	if !strings.Contains(err.Error(), "writing manifest YAML") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

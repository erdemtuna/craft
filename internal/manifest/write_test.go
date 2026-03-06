package manifest

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteRoundTrip(t *testing.T) {
	original := &Manifest{
		SchemaVersion: 1,
		Name:          "round-trip",
		Version:       "1.0.0",
		Description:   "Test round-trip.",
		License:       "MIT",
		Skills:        []string{"./skills/one", "./skills/two"},
		Dependencies: map[string]string{
			"dep-a": "github.com/org/repo-a@v1.0.0",
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
	if parsed.Version != original.Version {
		t.Errorf("Version: got %q, want %q", parsed.Version, original.Version)
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
	if parsed.Dependencies["dep-a"] != original.Dependencies["dep-a"] {
		t.Errorf("Dependencies[dep-a]: got %q, want %q", parsed.Dependencies["dep-a"], original.Dependencies["dep-a"])
	}
	if parsed.Metadata["author"] != original.Metadata["author"] {
		t.Errorf("Metadata[author]: got %q, want %q", parsed.Metadata["author"], original.Metadata["author"])
	}
}

func TestWriteFieldOrder(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "ordered",
		Version:       "1.0.0",
		Description:   "Test ordering.",
		Skills:        []string{"./skill"},
	}

	var buf bytes.Buffer
	if err := Write(m, &buf); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// schema_version should appear before name, which should appear before version
	schemaIdx := strings.Index(output, "schema_version")
	nameIdx := strings.Index(output, "\nname:")
	versionIdx := strings.Index(output, "\nversion:")
	skillsIdx := strings.Index(output, "\nskills:")

	if schemaIdx == -1 || nameIdx == -1 || versionIdx == -1 || skillsIdx == -1 {
		t.Fatalf("Missing expected fields in output:\n%s", output)
	}

	if schemaIdx >= nameIdx {
		t.Error("schema_version should appear before name")
	}
	if nameIdx >= versionIdx {
		t.Error("name should appear before version")
	}
	if versionIdx >= skillsIdx {
		t.Error("version should appear before skills")
	}
}

func TestWriteOmitsEmpty(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "minimal",
		Version:       "0.1.0",
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

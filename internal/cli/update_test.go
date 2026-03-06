package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/manifest"
)

func TestFindLatestSemverTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"basic", []string{"v1.0.0", "v1.1.0", "v0.9.0"}, "v1.1.0"},
		{"major_wins", []string{"v2.0.0", "v1.9.9"}, "v2.0.0"},
		{"patch_wins", []string{"v1.0.0", "v1.0.1", "v1.0.2"}, "v1.0.2"},
		{"single_tag", []string{"v3.2.1"}, "v3.2.1"},
		{"skip_non_semver", []string{"latest", "v1.0.0", "beta"}, "v1.0.0"},
		// Sscanf parses "1.0.0-alpha" as 1.0.0, so pre-release suffixes are treated as their base version
		{"prerelease_parsed_as_base", []string{"v1.0.0-alpha", "v0.9.0"}, "v1.0.0-alpha"},
		{"no_v_prefix_skipped", []string{"1.0.0", "2.0.0"}, ""},
		{"empty", []string{}, ""},
		{"nil", nil, ""},
		{"all_invalid", []string{"latest", "main", "dev"}, ""},
		{"mixed_valid_invalid", []string{"not-a-tag", "v1.2.3", "vx.y.z", "v2.0.0"}, "v2.0.0"},
		{"equal_versions", []string{"v1.0.0", "v1.0.0"}, "v1.0.0"},
		{"high_numbers", []string{"v100.200.300", "v1.0.0"}, "v100.200.300"},
		{"minor_tiebreaker", []string{"v1.2.0", "v1.3.0", "v1.1.0"}, "v1.3.0"},
		{"two_part_version", []string{"v1.0"}, ""}, // not valid semver (needs 3 parts)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findLatestSemverTag(tt.tags)
			if got != tt.want {
				t.Errorf("findLatestSemverTag(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestCompareParts(t *testing.T) {
	tests := []struct {
		name string
		a, b [3]int
		want int
	}{
		{"equal", [3]int{1, 2, 3}, [3]int{1, 2, 3}, 0},
		{"a_greater_major", [3]int{2, 0, 0}, [3]int{1, 9, 9}, 1},
		{"b_greater_major", [3]int{1, 0, 0}, [3]int{2, 0, 0}, -1},
		{"a_greater_minor", [3]int{1, 2, 0}, [3]int{1, 1, 9}, 1},
		{"b_greater_minor", [3]int{1, 1, 0}, [3]int{1, 2, 0}, -1},
		{"a_greater_patch", [3]int{1, 0, 2}, [3]int{1, 0, 1}, 1},
		{"b_greater_patch", [3]int{1, 0, 1}, [3]int{1, 0, 2}, -1},
		{"zeros", [3]int{0, 0, 0}, [3]int{0, 0, 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareParts(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareParts(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestWriteManifestAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "craft.yaml")

	m := &manifest.Manifest{
		SchemaVersion: 1,
		Name:          "test-pkg",
		Version:       "1.0.0",
		Skills:        []string{"skills/lint"},
		Dependencies: map[string]string{
			"tools": "github.com/org/tools@v1.0.0",
		},
	}

	if err := writeManifestAtomic(path, m); err != nil {
		t.Fatalf("writeManifestAtomic error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "test-pkg") {
		t.Errorf("manifest should contain package name, got:\n%s", content)
	}
	if !strings.Contains(content, "github.com/org/tools@v1.0.0") {
		t.Errorf("manifest should contain dependency URL, got:\n%s", content)
	}

	// Verify temp file was cleaned up
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after atomic write")
	}
}

func TestWriteManifestAtomic_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "craft.yaml")

	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		SchemaVersion: 1,
		Name:          "updated",
		Version:       "2.0.0",
		Skills:        []string{"skills/new"},
	}

	if err := writeManifestAtomic(path, m); err != nil {
		t.Fatalf("writeManifestAtomic error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) == "old" {
		t.Error("manifest should have been overwritten")
	}
	if !strings.Contains(string(data), "updated") {
		t.Error("manifest should contain new package name")
	}
}

func TestWriteManifestAtomic_BadPath(t *testing.T) {
	err := writeManifestAtomic("/nonexistent/dir/craft.yaml", &manifest.Manifest{
		SchemaVersion: 1,
		Name:          "test",
		Version:       "1.0.0",
	})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

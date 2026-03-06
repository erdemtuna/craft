package semver

import (
	"testing"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.0.0", "1.0.0", 0},
		{"a_greater_patch", "1.0.1", "1.0.0", 1},
		{"b_greater_patch", "1.0.0", "1.0.1", -1},
		{"a_greater_major", "2.0.0", "1.9.9", 1},
		{"a_greater_minor", "1.2.0", "1.1.9", 1},
		{"minimal_difference", "0.0.1", "0.0.0", 1},
		{"multi_digit", "10.20.30", "10.20.29", 1},
		{"zeros", "0.0.0", "0.0.0", 0},
		{"invalid_a", "abc", "1.0.0", -1},
		{"invalid_b", "1.0.0", "abc", 1},
		{"both_invalid", "abc", "xyz", 0},
		{"empty_strings", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseParts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [3]int
	}{
		{"valid", "1.2.3", [3]int{1, 2, 3}},
		{"zeros", "0.0.0", [3]int{0, 0, 0}},
		{"large_numbers", "100.200.300", [3]int{100, 200, 300}},
		{"invalid", "abc", [3]int{0, 0, 0}},
		{"empty", "", [3]int{0, 0, 0}},
		{"partial", "1.2", [3]int{1, 2, 0}},
		{"with_extra", "1.2.3.4", [3]int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseParts(tt.input)
			if got != tt.want {
				t.Errorf("ParseParts(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindLatest(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"basic_ordering", []string{"v1.0.0", "v1.1.0", "v0.9.0"}, "v1.1.0"},
		{"major_wins", []string{"v2.0.0", "v1.9.9"}, "v2.0.0"},
		{"patch_wins", []string{"v1.0.0", "v1.0.1", "v1.0.2"}, "v1.0.2"},
		{"single_tag", []string{"v3.2.1"}, "v3.2.1"},
		{"skip_non_semver", []string{"latest", "v1.0.0", "beta"}, "v1.0.0"},
		{"prerelease_rejected", []string{"v1.0.0-alpha", "v0.9.0"}, "v0.9.0"},
		{"prerelease_beta_rejected", []string{"v2.0.0-beta.1", "v1.5.0"}, "v1.5.0"},
		{"build_metadata_rejected", []string{"v1.0.0+build", "v0.9.0"}, "v0.9.0"},
		{"no_v_prefix_skipped", []string{"1.0.0", "2.0.0"}, ""},
		{"empty", []string{}, ""},
		{"nil", nil, ""},
		{"all_invalid", []string{"latest", "main", "dev"}, ""},
		{"mixed_valid_invalid", []string{"not-a-tag", "v1.2.3", "vx.y.z", "v2.0.0"}, "v2.0.0"},
		{"equal_versions", []string{"v1.0.0", "v1.0.0"}, "v1.0.0"},
		{"high_numbers", []string{"v100.200.300", "v1.0.0"}, "v100.200.300"},
		{"minor_tiebreaker", []string{"v1.2.0", "v1.3.0", "v1.1.0"}, "v1.3.0"},
		{"two_part_version", []string{"v1.0"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindLatest(tt.tags)
			if got != tt.want {
				t.Errorf("FindLatest(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

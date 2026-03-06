// Package semver provides basic semantic versioning utilities for craft.
package semver

import (
	"fmt"
	"strings"
)

// Compare compares two semver version strings (without v prefix).
// Returns -1, 0, or 1. Returns 0 for invalid inputs (treat as 0.0.0).
func Compare(a, b string) int {
	aParts := ParseParts(a)
	bParts := ParseParts(b)
	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return 1
		}
		if aParts[i] < bParts[i] {
			return -1
		}
	}
	return 0
}

// ParseParts splits "1.2.3" into [3]int{1, 2, 3}.
// Returns [0, 0, 0] for unparseable input.
func ParseParts(v string) [3]int {
	var parts [3]int
	fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}

// FindLatest returns the highest semver tag from a list of git tags.
// Tags must start with 'v' and be valid MAJOR.MINOR.PATCH (no pre-release).
// Returns empty string if no valid semver tag is found.
func FindLatest(tags []string) string {
	var latest string
	var latestParts [3]int

	for _, tag := range tags {
		if len(tag) < 2 || tag[0] != 'v' {
			continue
		}
		version := tag[1:]
		// Reject pre-release and build metadata
		if strings.ContainsAny(version, "-+") {
			continue
		}
		var parts [3]int
		n, _ := fmt.Sscanf(version, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
		if n != 3 {
			continue
		}

		if latest == "" || compareParts(parts, latestParts) > 0 {
			latest = tag
			latestParts = parts
		}
	}

	return latest
}

func compareParts(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

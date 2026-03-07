package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// DepNode represents a resolved dependency for tree display.
type DepNode struct {
	Alias  string
	URL    string
	Skills []string
}

// RenderTree writes a box-drawing dependency tree to w.
// localSkills are shown first under "Local skills:", then remote dependencies.
func RenderTree(w io.Writer, packageName string, localSkills []string, deps []DepNode) {
	_, _ = fmt.Fprintf(w, "%s\n", packageName)

	hasLocal := len(localSkills) > 0
	hasDeps := len(deps) > 0

	if hasLocal {
		connector := "├── "
		childPrefix := "│   "
		if !hasDeps {
			connector = "└── "
			childPrefix = "    "
		}
		_, _ = fmt.Fprintf(w, "%sLocal skills\n", connector)
		for i, skill := range localSkills {
			skillConn := "├── "
			if i == len(localSkills)-1 {
				skillConn = "└── "
			}
			_, _ = fmt.Fprintf(w, "%s%s%s\n", childPrefix, skillConn, skill)
		}
	}

	// Sort deps by alias for deterministic output
	sorted := make([]DepNode, len(deps))
	copy(sorted, deps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Alias < sorted[j].Alias
	})

	for i, dep := range sorted {
		isLast := i == len(sorted)-1
		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		_, _ = fmt.Fprintf(w, "%s%s (%s)\n", connector, dep.Alias, dep.URL)

		for j, skill := range dep.Skills {
			skillConn := "├── "
			if j == len(dep.Skills)-1 {
				skillConn = "└── "
			}
			_, _ = fmt.Fprintf(w, "%s%s%s\n", childPrefix, skillConn, skill)
		}
	}
}

// FormatTree renders the tree to a string for convenience.
func FormatTree(packageName string, localSkills []string, deps []DepNode) string {
	var sb strings.Builder
	RenderTree(&sb, packageName, localSkills, deps)
	return sb.String()
}

// Package skill defines types and parsing for SKILL.md frontmatter.
package skill

// Frontmatter represents the YAML frontmatter extracted from a SKILL.md file.
type Frontmatter struct {
	// Name is the skill identifier (required, lowercase alphanumeric with hyphens).
	Name string `yaml:"name"`

	// Description is an optional human-readable skill description.
	Description string `yaml:"description,omitempty"`

	// Extra holds any additional frontmatter fields for forward compatibility.
	// These are parsed but not validated by craft.
	Extra map[string]interface{} `yaml:"-"`
}

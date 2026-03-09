// Package pinfile defines the craft.pin.yaml pinfile types for parsing
// and validation.
package pinfile

// Pinfile represents a craft.pin.yaml resolved dependency file.
type Pinfile struct {
	// PinVersion is the pinfile schema version (always 1 for this release).
	PinVersion int `yaml:"pin_version"`

	// Resolved maps dependency URLs to their resolved state.
	// Keys are the dependency URL values from the manifest (e.g., "github.com/example/skills@v1.0.0").
	Resolved map[string]ResolvedEntry `yaml:"resolved"`
}

// ResolvedEntry holds the resolved state for a single dependency.
type ResolvedEntry struct {
	// Commit is the full git commit SHA the dependency resolved to.
	Commit string `yaml:"commit"`

	// RefType indicates the kind of reference: "tag", "commit", or "branch".
	// Empty or absent defaults to "tag" for backward compatibility.
	RefType string `yaml:"ref_type,omitempty"`

	// Integrity is the SHA-256 integrity digest of the dependency content.
	Integrity string `yaml:"integrity"`

	// Source is the dependency URL of the parent package that declared this
	// dependency. Empty for direct dependencies, set for transitive entries.
	Source string `yaml:"source,omitempty"`

	// Skills lists the skill names discovered in the dependency.
	Skills []string `yaml:"skills"`

	// SkillPaths lists the skill directory paths relative to the repo root.
	SkillPaths []string `yaml:"skill_paths,omitempty"`
}

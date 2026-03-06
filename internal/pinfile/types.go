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

	// Integrity is the SHA-256 integrity digest of the dependency content.
	Integrity string `yaml:"integrity"`

	// Skills lists the skill names discovered in the dependency.
	Skills []string `yaml:"skills"`
}

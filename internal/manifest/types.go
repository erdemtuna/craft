// Package manifest defines the craft.yaml manifest types for parsing,
// validation, and serialization.
package manifest

// Manifest represents a craft.yaml package manifest.
type Manifest struct {
	// SchemaVersion is the manifest schema version (always 1 for this release).
	SchemaVersion int `yaml:"schema_version"`

	// Name is the package name (lowercase alphanumeric with hyphens).
	Name string `yaml:"name"`

	// Description is an optional human-readable package description.
	Description string `yaml:"description,omitempty"`

	// License is an optional SPDX license identifier.
	License string `yaml:"license,omitempty"`

	// Skills is the list of relative paths to skill directories.
	Skills []string `yaml:"skills"`

	// Dependencies maps aliases to dependency URLs (host/org/repo@vMAJOR.MINOR.PATCH).
	Dependencies map[string]string `yaml:"dependencies,omitempty"`

	// Metadata holds arbitrary key-value pairs for extensibility.
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

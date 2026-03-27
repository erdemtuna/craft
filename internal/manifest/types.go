// Package manifest defines the craft.yaml manifest types for parsing,
// validation, and serialization.
package manifest

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DependencySpec represents a dependency in craft.yaml.
// It can be either a simple URL string or a structured object with url and select fields.
type DependencySpec struct {
	// URL is the dependency URL (host/org/repo@ref).
	URL string
	// Select is an optional list of skill subpaths to install from this dependency.
	// When empty, all exported skills are installed.
	Select []string
}

// UnmarshalYAML implements custom YAML unmarshaling to handle both string
// and object forms of dependency declarations.
func (d *DependencySpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		d.URL = value.Value
		return nil
	}
	if value.Kind == yaml.MappingNode {
		type raw struct {
			URL    string   `yaml:"url"`
			Select []string `yaml:"select"`
		}
		var r raw
		if err := value.Decode(&r); err != nil {
			return err
		}
		if r.URL == "" {
			return fmt.Errorf("structured dependency requires 'url' field")
		}
		d.URL = r.URL
		d.Select = normalizeSelectPaths(r.Select)
		return nil
	}
	return fmt.Errorf("dependency must be a string or object, got %v", value.Kind)
}

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

	// Dependencies maps aliases to dependency specifications.
	Dependencies map[string]DependencySpec `yaml:"dependencies,omitempty"`

	// Metadata holds arbitrary key-value pairs for extensibility.
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

// normalizeSelectPaths strips leading "./" and trailing "/" from each select path.
func normalizeSelectPaths(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimSuffix(p, "/")
		out[i] = p
	}
	return out
}

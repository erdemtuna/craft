package manifest

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a craft.yaml manifest from the given reader.
// Unknown fields are silently accepted for forward compatibility.
func Parse(r io.Reader) (*Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}

	return &m, nil
}

// ParseFile reads a craft.yaml manifest from the given file path.
func ParseFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening manifest file: %w", err)
	}
	defer func() { _ = f.Close() }()

	return Parse(f)
}

package pinfile

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a craft.pin.yaml pinfile from the given reader.
func Parse(r io.Reader) (*Pinfile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading pinfile: %w", err)
	}

	var p Pinfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing pinfile YAML: %w", err)
	}

	// Default empty RefType to "tag" for backward compatibility with
	// pinfiles created before non-tagged dependency support.
	for url, entry := range p.Resolved {
		if entry.RefType == "" {
			entry.RefType = "tag"
			p.Resolved[url] = entry
		}
	}

	return &p, nil
}

// ParseFile reads a craft.pin.yaml pinfile from the given file path.
func ParseFile(path string) (*Pinfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening pinfile: %w", err)
	}
	defer func() { _ = f.Close() }()

	return Parse(f)
}

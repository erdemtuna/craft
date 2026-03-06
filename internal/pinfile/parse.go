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

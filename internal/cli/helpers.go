package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
)

// requireManifestAndPinfile parses both craft.yaml and craft.pin.yaml from the
// current working directory. It returns a user-friendly error if either file
// is missing — callers should not proceed without both files.
func requireManifestAndPinfile() (*manifest.Manifest, *pinfile.Pinfile, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("getting working directory: %w", err)
	}

	m, err := manifest.ParseFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("craft.yaml not found\n  hint: run `craft init` to create one")
		}
		return nil, nil, fmt.Errorf("reading craft.yaml: %w", err)
	}

	pf, err := pinfile.ParseFile(filepath.Join(root, "craft.pin.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("craft.pin.yaml not found\n  hint: run `craft install` to resolve and pin dependencies")
		}
		return nil, nil, fmt.Errorf("reading craft.pin.yaml: %w", err)
	}

	return m, pf, nil
}

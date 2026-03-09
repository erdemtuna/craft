package cli

import (
	"os"
	"path/filepath"
)

var globalFlag bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&globalFlag, "global", "g", false, "Operate on the global (~/.craft/) skill store")
}

// GlobalCraftDir returns the path to the global craft directory (~/.craft/).
func GlobalCraftDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".craft"), nil
}

// GlobalManifestPath returns the path to the global craft manifest (~/.craft/craft.yaml).
func GlobalManifestPath() (string, error) {
	dir, err := GlobalCraftDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "craft.yaml"), nil
}

// GlobalPinfilePath returns the path to the global craft pinfile (~/.craft/craft.pin.yaml).
func GlobalPinfilePath() (string, error) {
	dir, err := GlobalCraftDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "craft.pin.yaml"), nil
}

func ensureGlobalDir() error {
	dir, err := GlobalCraftDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

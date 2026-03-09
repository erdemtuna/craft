package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalCraftDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := GlobalCraftDir()
	if err != nil {
		t.Fatalf("GlobalCraftDir: %v", err)
	}

	want := filepath.Join(home, ".craft")
	if got != want {
		t.Errorf("GlobalCraftDir() = %q, want %q", got, want)
	}
}

func TestGlobalManifestPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := GlobalManifestPath()
	if err != nil {
		t.Fatalf("GlobalManifestPath: %v", err)
	}

	want := filepath.Join(home, ".craft", "craft.yaml")
	if got != want {
		t.Errorf("GlobalManifestPath() = %q, want %q", got, want)
	}
}

func TestGlobalPinfilePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := GlobalPinfilePath()
	if err != nil {
		t.Fatalf("GlobalPinfilePath: %v", err)
	}

	want := filepath.Join(home, ".craft", "craft.pin.yaml")
	if got != want {
		t.Errorf("GlobalPinfilePath() = %q, want %q", got, want)
	}
}

func TestGlobalFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("global")
	if f == nil {
		t.Fatal("expected --global flag to be registered on rootCmd")
	}
	if f.Shorthand != "g" {
		t.Errorf("expected shorthand 'g', got %q", f.Shorthand)
	}
	if f.DefValue != "false" {
		t.Errorf("expected default value 'false', got %q", f.DefValue)
	}
}

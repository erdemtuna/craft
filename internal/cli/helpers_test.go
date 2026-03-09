package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestPathForScope(t *testing.T) {
	t.Run("project scope", func(t *testing.T) {
		saved := globalFlag
		t.Cleanup(func() { globalFlag = saved })
		globalFlag = false

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		mPath, pPath, err := manifestPathForScope()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantManifest := filepath.Join(cwd, "craft.yaml")
		wantPinfile := filepath.Join(cwd, "craft.pin.yaml")
		if mPath != wantManifest {
			t.Errorf("manifestPath = %q, want %q", mPath, wantManifest)
		}
		if pPath != wantPinfile {
			t.Errorf("pinfilePath = %q, want %q", pPath, wantPinfile)
		}
	})

	t.Run("global scope", func(t *testing.T) {
		saved := globalFlag
		t.Cleanup(func() { globalFlag = saved })
		globalFlag = true

		mPath, pPath, err := manifestPathForScope()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		wantManifest := filepath.Join(home, ".craft", "craft.yaml")
		wantPinfile := filepath.Join(home, ".craft", "craft.pin.yaml")
		if mPath != wantManifest {
			t.Errorf("manifestPath = %q, want %q", mPath, wantManifest)
		}
		if pPath != wantPinfile {
			t.Errorf("pinfilePath = %q, want %q", pPath, wantPinfile)
		}
	})
}

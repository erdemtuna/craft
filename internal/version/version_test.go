package version

import "testing"

func TestDefaultVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
	if Version != "dev" {
		t.Errorf("Default version should be \"dev\", got %q", Version)
	}
}

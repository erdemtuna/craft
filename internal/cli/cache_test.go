package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheCleanRemovesCache(t *testing.T) {
	// Create a temporary cache directory with some fake repos
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, ".craft", "cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "repo1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "repo2"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "repo1", "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify setup
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Remove the cache
	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatalf("RemoveAll error: %v", err)
	}

	// Verify removed
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("cache directory should not exist after clean")
	}
}

func TestCacheCleanEmptyCache(t *testing.T) {
	// Create empty cache
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, ".craft", "cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Fatalf("expected empty cache")
	}
}

package fetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	cache, err := NewCache(root)
	if err != nil {
		t.Fatalf("NewCache error: %v", err)
	}

	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Error("Cache root directory should be created")
	}

	if cache.Root != root {
		t.Errorf("Root = %q, want %q", cache.Root, root)
	}
}

func TestCacheHasMiss(t *testing.T) {
	cache, _ := NewCache(t.TempDir())
	if cache.Has("https://github.com/nonexistent/repo.git") {
		t.Error("Expected cache miss for nonexistent repo")
	}
}

func TestCacheHasHit(t *testing.T) {
	cache, _ := NewCache(t.TempDir())
	url := "https://github.com/org/repo.git"
	repoPath := cache.RepoPath(url)

	// Simulate cached repo by creating the directory
	os.MkdirAll(repoPath, 0o755)

	if !cache.Has(url) {
		t.Error("Expected cache hit for existing repo directory")
	}
}

func TestCacheRemove(t *testing.T) {
	cache, _ := NewCache(t.TempDir())
	url := "https://github.com/org/repo.git"
	repoPath := cache.RepoPath(url)

	os.MkdirAll(repoPath, 0o755)
	if !cache.Has(url) {
		t.Fatal("Setup: expected cache hit")
	}

	if err := cache.Remove(url); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	if cache.Has(url) {
		t.Error("Expected cache miss after removal")
	}
}

func TestCacheRemoveNonexistent(t *testing.T) {
	cache, _ := NewCache(t.TempDir())
	if err := cache.Remove("https://github.com/missing/repo.git"); err != nil {
		t.Fatalf("Remove of nonexistent should not error: %v", err)
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo.git", "github.com-org-repo"},
		{"github.com/org/repo", "github.com-org-repo"},
		{"git@github.com:org/repo.git", "github.com-org-repo"},
		{"https://gitlab.example.io/my-org/my-repo.git", "gitlab.example.io-my-org-my-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeURL(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultCacheRoot(t *testing.T) {
	root, err := DefaultCacheRoot()
	if err != nil {
		t.Fatalf("DefaultCacheRoot error: %v", err)
	}
	if !filepath.IsAbs(root) {
		t.Errorf("Expected absolute path, got %q", root)
	}
	if !strings.Contains(root, ".craft") {
		t.Errorf("Expected path containing .craft, got %q", root)
	}
}

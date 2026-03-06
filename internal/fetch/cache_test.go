package fetch

import (
	"encoding/hex"
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
	const orgRepoHash = "4c06e3f1e1c41311585c1ae6798e89d80713d0205bce65835c4682eb1474f7b8"
	const gitlabHash = "bcc37343d23b16e1aa029442d9295e9ad8f30dc46d2cb83b7aed487bc6d7756f"

	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo.git", orgRepoHash},
		{"github.com/org/repo", orgRepoHash},
		{"git@github.com:org/repo.git", orgRepoHash},
		{"https://gitlab.example.io/my-org/my-repo.git", gitlabHash},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeURL(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) != 64 {
				t.Errorf("expected 64-char hex string, got length %d", len(got))
			}
			if _, err := hex.DecodeString(got); err != nil {
				t.Errorf("result is not valid hex: %v", err)
			}
		})
	}
}

func TestSanitizeURLCollisionFree(t *testing.T) {
	// These two URLs previously collided (both mapped to "github.com-org-name-repo")
	a := sanitizeURL("https://github.com/org-name/repo.git")
	b := sanitizeURL("https://github.com/org/name-repo.git")

	if a == b {
		t.Errorf("collision detected: %q and %q both hash to %q",
			"github.com/org-name/repo", "github.com/org/name-repo", a)
	}

	// Verify both are valid hex of correct length
	for _, h := range []string{a, b} {
		if len(h) != 64 {
			t.Errorf("expected 64-char hex string, got length %d: %s", len(h), h)
		}
		if _, err := hex.DecodeString(h); err != nil {
			t.Errorf("result is not valid hex: %v", err)
		}
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

package fetch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache manages a local cache of bare git repositories at a configurable
// root directory (default: ~/.craft/cache/).
type Cache struct {
	// Root is the cache root directory.
	Root string
}

// NewCache creates a Cache with the given root directory.
// Creates the root directory if it does not exist.
func NewCache(root string) (*Cache, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	return &Cache{Root: root}, nil
}

// DefaultCacheRoot returns the default cache directory path (~/.craft/cache/).
func DefaultCacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".craft", "cache"), nil
}

// RepoPath returns the cache path for a given repository URL.
// Sanitizes the URL to produce a filesystem-safe directory name.
func (c *Cache) RepoPath(repoURL string) string {
	return filepath.Join(c.Root, sanitizeURL(repoURL))
}

// Has checks if a repository exists in the cache.
func (c *Cache) Has(repoURL string) bool {
	path := c.RepoPath(repoURL)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// sanitizeURL converts a git URL to a filesystem-safe directory name.
// "https://github.com/org/repo.git" → "github.com-org-repo"
// "github.com/org/repo" → "github.com-org-repo"
func sanitizeURL(url string) string {
	// Strip protocol
	s := url
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@")
	s = strings.TrimPrefix(s, "ssh://")

	// Strip .git suffix
	s = strings.TrimSuffix(s, ".git")

	// Replace separators with hyphens
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")

	return s
}

package fetch

import (
	"crypto/sha256"
	"encoding/hex"
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
	if err := os.MkdirAll(root, 0o700); err != nil {
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
// Uses SHA-256 hash of the normalized URL to avoid collisions.
func sanitizeURL(rawURL string) string {
	// Normalize: strip protocol and .git suffix to get a canonical identity
	s := rawURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "git@")
	s = strings.TrimPrefix(s, "ssh://")
	s = strings.TrimSuffix(s, ".git")
	s = strings.ReplaceAll(s, ":", "/") // normalize git@ style

	// Use SHA-256 hash to avoid collisions between similarly-named repos
	// (e.g., org-name/repo vs org/name-repo)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

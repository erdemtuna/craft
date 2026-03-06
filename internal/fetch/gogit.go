package fetch

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

const defaultGitTimeout = 2 * time.Minute

// GoGitFetcher implements GitFetcher using go-git with a local bare clone cache.
type GoGitFetcher struct {
	cache   *Cache
	timeout time.Duration
	offline bool // skip network fetches, use cached data only
}

// NewGoGitFetcher creates a GoGitFetcher backed by the given cache.
func NewGoGitFetcher(cache *Cache) *GoGitFetcher {
	return &GoGitFetcher{cache: cache, timeout: defaultGitTimeout}
}

// SetOffline configures the fetcher to skip network operations and use
// cached data only. Clones still occur on cache miss — offline mode
// only suppresses fetch-on-hit to avoid unnecessary network round-trips.
func (f *GoGitFetcher) SetOffline(offline bool) {
	f.offline = offline
}

// ResolveRef resolves a git tag or branch name to a full commit SHA.
func (f *GoGitFetcher) ResolveRef(url, ref string) (string, error) {
	repo, err := f.ensureRepo(url)
	if err != nil {
		return "", WrapAuthError(fmt.Errorf("resolving ref %q from %s: %w", ref, url, err), url)
	}

	// Try as tag first
	tagRef, err := repo.Tag(ref)
	if err == nil {
		// Could be a lightweight tag (points directly to commit) or
		// an annotated tag (points to tag object).
		obj, err := repo.TagObject(tagRef.Hash())
		if err == nil {
			// Annotated tag — dereference to commit
			commit, err := obj.Commit()
			if err != nil {
				return "", fmt.Errorf("dereferencing annotated tag %q: %w", ref, err)
			}
			return commit.Hash.String(), nil
		}
		// Lightweight tag — hash is the commit itself
		return tagRef.Hash().String(), nil
	}

	// Try as branch
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(ref), true)
	if err == nil {
		return branchRef.Hash().String(), nil
	}

	// Try as remote branch
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", ref), true)
	if err == nil {
		return remoteRef.Hash().String(), nil
	}

	return "", fmt.Errorf("ref %q not found in %s — is the tag or branch name correct?", ref, url)
}

// ListTags returns all tag names from the repository.
func (f *GoGitFetcher) ListTags(url string) ([]string, error) {
	repo, err := f.ensureRepo(url)
	if err != nil {
		return nil, WrapAuthError(fmt.Errorf("listing tags from %s: %w", url, err), url)
	}

	iter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	var tags []string
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		tags = append(tags, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}

	return tags, nil
}

// ListTree returns all file paths in the repository tree at the given commit.
func (f *GoGitFetcher) ListTree(url, commitSHA string) ([]string, error) {
	repo, err := f.ensureRepo(url)
	if err != nil {
		return nil, WrapAuthError(fmt.Errorf("listing tree from %s: %w", url, err), url)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return nil, fmt.Errorf("commit %s not found: %w", commitSHA, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("reading tree: %w", err)
	}

	var paths []string
	err = tree.Files().ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking tree: %w", err)
	}

	return paths, nil
}

// ReadFiles reads the contents of specific files at a given commit.
// Missing paths are silently skipped.
func (f *GoGitFetcher) ReadFiles(url, commitSHA string, paths []string) (map[string][]byte, error) {
	repo, err := f.ensureRepo(url)
	if err != nil {
		return nil, WrapAuthError(fmt.Errorf("reading files from %s: %w", url, err), url)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return nil, fmt.Errorf("commit %s not found: %w", commitSHA, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("reading tree: %w", err)
	}

	result := make(map[string][]byte)
	for _, p := range paths {
		file, err := tree.File(p)
		if err != nil {
			continue // silently skip missing files
		}
		reader, err := file.Reader()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			continue
		}
		result[p] = content
	}

	return result, nil
}

// ensureRepo returns a go-git Repository for the given URL, using the cache.
// On cache miss, clones the repo as a bare clone. On cache hit, fetches
// latest changes.
func (f *GoGitFetcher) ensureRepo(url string) (*git.Repository, error) {
	if f.cache == nil {
		return f.cloneToMemory(url)
	}

	repoPath := f.cache.RepoPath(url)

	// Acquire per-repo lock to prevent concurrent corruption
	lock, err := lockRepo(repoPath)
	if err != nil {
		// If locking fails, proceed without lock (best-effort)
		// This handles systems where flock is unavailable
		lock = nil
	}
	defer func() {
		if lock != nil {
			lock.Unlock()
		}
	}()

	if f.cache.Has(url) {
		// Cache hit — open and fetch
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			// Corrupted cache — remove and re-clone
			_ = os.RemoveAll(repoPath)
			return f.cloneToCache(url, repoPath)
		}

		if f.offline {
			return repo, nil
		}

		// Fetch latest (best-effort — if offline, use existing)
		auth := Auth(url)
		ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
		defer cancel()
		err = repo.FetchContext(ctx, &git.FetchOptions{
			RemoteURL: url,
			Auth:      auth,
			Tags:      git.AllTags,
			Force:     true,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			// Auth failures should propagate — stale cache with bad credentials
			// likely means the user's token expired, not a network issue
			if isAuthError(err) {
				return nil, WrapAuthError(fmt.Errorf("fetching %s: %w", url, err), url)
			}
			// Network/timeout failures — use existing cached data (offline fallback)
		}

		return repo, nil
	}

	// Cache miss — clone
	return f.cloneToCache(url, repoPath)
}

// cloneToCache performs an atomic bare clone to the cache directory.
func (f *GoGitFetcher) cloneToCache(url, repoPath string) (*git.Repository, error) {
	auth := Auth(url)

	// Atomic clone: write to temp dir in same filesystem, then rename
	tmpDir, err := os.MkdirTemp(f.cache.Root, "clone-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir for clone: %w", err)
	}
	defer func() {
		// Clean up temp dir if it still exists (rename succeeded → no-op)
		_ = os.RemoveAll(tmpDir)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()
	_, err = git.PlainCloneContext(ctx, tmpDir, true, &git.CloneOptions{
		URL:  url,
		Auth: auth,
		Tags: git.AllTags,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning %s: %w", url, err)
	}

	// Atomic rename
	if err := os.Rename(tmpDir, repoPath); err != nil {
		// Rename failed — another process may have won the race.
		// Try to open the existing path.
		if existing, openErr := git.PlainOpen(repoPath); openErr == nil {
			return existing, nil
		}
		return nil, fmt.Errorf("storing cached repo: %w", err)
	}

	// Re-open from final path
	return git.PlainOpen(repoPath)
}

// cloneToMemory clones a repo into memory (no cache).
func (f *GoGitFetcher) cloneToMemory(url string) (*git.Repository, error) {
	auth := Auth(url)
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()
	repo, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
		URL:  url,
		Auth: auth,
		Tags: git.AllTags,
	})
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// isAuthError returns true if the error message suggests an authentication failure.
func isAuthError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, keyword := range []string{"authentication", "denied", "unauthorized", "403", "401"} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

// NormalizeCloneURL converts a dependency package identity to a clone URL.
// For hosts that look like common git providers, uses HTTPS by default.
func NormalizeCloneURL(packageIdentity string) string {
	// If it already looks like a full URL, use as-is
	if strings.HasPrefix(packageIdentity, "https://") || strings.HasPrefix(packageIdentity, "git@") {
		return packageIdentity
	}

	return "https://" + packageIdentity + ".git"
}

// Remove deletes a cached repository (used for cache invalidation).
func (c *Cache) Remove(repoURL string) error {
	path := c.RepoPath(repoURL)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(path)
}

// Package fetch provides git repository fetching with caching and
// authentication for craft dependency resolution.
package fetch

// GitFetcher abstracts git operations for testability. The interface
// decouples resolution logic from the concrete go-git implementation.
type GitFetcher interface {
	// ResolveRef resolves a git tag or branch name to a full commit SHA.
	ResolveRef(url, ref string) (commitSHA string, err error)

	// ListTags returns all tag names from the remote repository.
	ListTags(url string) ([]string, error)

	// ListTree returns all file paths in the repository tree at the
	// given commit SHA. Used for auto-discovery of SKILL.md files in
	// dependencies without craft.yaml.
	ListTree(url, commitSHA string) ([]string, error)

	// ReadFiles reads the contents of specific files at a given commit.
	// Returns a map of path → content. Missing paths are silently skipped.
	ReadFiles(url, commitSHA string, paths []string) (map[string][]byte, error)
}

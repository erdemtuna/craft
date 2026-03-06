package fetch

import (
	"fmt"
	"testing"
)

func TestNormalizeCloneURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/org/repo", "https://github.com/org/repo.git"},
		{"https://github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"git@github.com:org/repo.git", "git@github.com:org/repo.git"},
		{"gitlab.example.io/my-org/my-repo", "https://gitlab.example.io/my-org/my-repo.git"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeCloneURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeCloneURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// MockFetcher implements GitFetcher for testing resolution logic
// without network access.
type MockFetcher struct {
	// Refs maps url+ref to commit SHA
	Refs map[string]string

	// Tags maps url to tag list
	Tags map[string][]string

	// Trees maps url+commitSHA to file path list
	Trees map[string][]string

	// Files maps url+commitSHA+path to file content
	Files map[string][]byte
}

// NewMockFetcher creates an empty MockFetcher.
func NewMockFetcher() *MockFetcher {
	return &MockFetcher{
		Refs:  make(map[string]string),
		Tags:  make(map[string][]string),
		Trees: make(map[string][]string),
		Files: make(map[string][]byte),
	}
}

func (m *MockFetcher) ResolveRef(url, ref string) (string, error) {
	key := url + ":" + ref
	if sha, ok := m.Refs[key]; ok {
		return sha, nil
	}
	return "", fmt.Errorf("ref %q not found in %s", ref, url)
}

func (m *MockFetcher) ListTags(url string) ([]string, error) {
	if tags, ok := m.Tags[url]; ok {
		return tags, nil
	}
	return nil, nil
}

func (m *MockFetcher) ListTree(url, commitSHA string) ([]string, error) {
	key := url + ":" + commitSHA
	if tree, ok := m.Trees[key]; ok {
		return tree, nil
	}
	return nil, nil
}

func (m *MockFetcher) ReadFiles(url, commitSHA string, paths []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, p := range paths {
		key := url + ":" + commitSHA + ":" + p
		if content, ok := m.Files[key]; ok {
			result[p] = content
		}
	}
	return result, nil
}

func TestMockFetcherImplementsInterface(t *testing.T) {
	var _ GitFetcher = (*MockFetcher)(nil)
}

func TestGoGitFetcherImplementsInterface(t *testing.T) {
	var _ GitFetcher = (*GoGitFetcher)(nil)
}

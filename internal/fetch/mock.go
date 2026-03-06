package fetch

import "fmt"

// MockFetcher implements GitFetcher for testing resolution logic
// without network access.
type MockFetcher struct {
	// Refs maps "url:ref" to commit SHA.
	Refs map[string]string

	// TagsByURL maps url to tag list.
	TagsByURL map[string][]string

	// Trees maps "url:commitSHA" to file path list.
	Trees map[string][]string

	// Files maps "url:commitSHA:path" to file content.
	Files map[string][]byte
}

// NewMockFetcher creates an empty MockFetcher.
func NewMockFetcher() *MockFetcher {
	return &MockFetcher{
		Refs:      make(map[string]string),
		TagsByURL: make(map[string][]string),
		Trees:     make(map[string][]string),
		Files:     make(map[string][]byte),
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
	if tags, ok := m.TagsByURL[url]; ok {
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

package fetch

import (
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

func TestMockFetcherImplementsInterface(t *testing.T) {
	var _ GitFetcher = (*MockFetcher)(nil)
}

func TestGoGitFetcherImplementsInterface(t *testing.T) {
	var _ GitFetcher = (*GoGitFetcher)(nil)
}

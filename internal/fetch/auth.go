package fetch

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// gitHubHosts lists hosts where GITHUB_TOKEN may be sent.
var gitHubHosts = []string{"github.com"}

// isGitHubHost returns true if the URL points to a known GitHub host.
func isGitHubHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, h := range gitHubHosts {
		if host == h {
			return true
		}
	}
	return false
}

// Auth resolves authentication for git operations.
// Checks environment tokens first (CRAFT_TOKEN > GITHUB_TOKEN),
// then falls back to SSH agent.
//
// CRAFT_TOKEN is sent to any host (user explicitly configured it for craft).
// GITHUB_TOKEN is only sent to known GitHub hosts to prevent token leakage
// to untrusted third-party servers via transitive dependencies.
//
// Returns nil auth (anonymous) if no credentials are found — the caller
// should attempt the operation and wrap any auth errors with suggestions.
func Auth(url string) transport.AuthMethod {
	// CRAFT_TOKEN: universal, sent to any host
	if token := os.Getenv("CRAFT_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}
	// GITHUB_TOKEN: scoped to GitHub hosts only
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && isGitHubHost(url) {
		return &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	// SSH agent (only for SSH URLs)
	if strings.HasPrefix(url, "git@") || strings.Contains(url, "ssh://") {
		if agent, err := ssh.NewSSHAgentAuth("git"); err == nil {
			return agent
		}
	}

	return nil
}

// WrapAuthError wraps a git operation error with actionable authentication
// suggestions if it appears to be an auth failure.
func WrapAuthError(err error, url string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "authentication") ||
		strings.Contains(errMsg, "denied") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "could not read") ||
		strings.Contains(errMsg, "repository not found") {
		return fmt.Errorf("%w\n  hint: is this a private repository? Set GITHUB_TOKEN or CRAFT_TOKEN, or configure SSH keys", err)
	}

	return err
}

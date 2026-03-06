package fetch

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// Auth resolves authentication for git operations.
// Checks environment tokens first (CRAFT_TOKEN > GITHUB_TOKEN),
// then falls back to SSH agent.
//
// Returns nil auth (anonymous) if no credentials are found — the caller
// should attempt the operation and wrap any auth errors with suggestions.
func Auth(url string) transport.AuthMethod {
	// Token auth via HTTPS
	if token := os.Getenv("CRAFT_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
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

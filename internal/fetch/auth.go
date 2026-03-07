package fetch

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// gitHubHosts lists hosts where GITHUB_TOKEN may be sent.
var gitHubHosts = []string{"github.com"}

// wellKnownForges lists hosts that are trusted by default for CRAFT_TOKEN
// when CRAFT_TOKEN_HOSTS is not set.
var wellKnownForges = []string{"github.com", "gitlab.com", "bitbucket.org"}

var craftTokenWarning sync.Once

// warnWriter is the writer used for warnings (stderr by default).
// Overridden in tests to capture output.
var warnWriter io.Writer = os.Stderr

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

// isTrustedHost returns true if rawURL points to a host that should receive
// CRAFT_TOKEN. When CRAFT_TOKEN_HOSTS is set, only those hosts are trusted.
// Otherwise, well-known forges are trusted and a one-time warning is emitted
// for any other host.
func isTrustedHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())

	// If CRAFT_TOKEN_HOSTS is set, use it as an explicit allowlist.
	if allowlist := os.Getenv("CRAFT_TOKEN_HOSTS"); allowlist != "" {
		for _, h := range strings.Split(allowlist, ",") {
			if strings.TrimSpace(strings.ToLower(h)) == host {
				return true
			}
		}
		return false
	}

	// No allowlist — check well-known forges.
	for _, h := range wellKnownForges {
		if host == h {
			return true
		}
	}

	// Unknown host without allowlist: warn once, then allow (backward compat).
	craftTokenWarning.Do(func() {
		fmt.Fprintf(warnWriter,
			"warning: CRAFT_TOKEN is being sent to %q which is not a well-known forge.\n"+
				"  Set CRAFT_TOKEN_HOSTS to restrict token to trusted hosts.\n", host)
	})
	return true
}

// Auth resolves authentication for git operations.
// Checks environment tokens first (CRAFT_TOKEN > GITHUB_TOKEN),
// then falls back to SSH agent.
//
// CRAFT_TOKEN is scoped to trusted hosts: well-known forges by default, or
// hosts listed in CRAFT_TOKEN_HOSTS when set. A one-time warning is emitted
// when the token is sent to an unknown host without an explicit allowlist.
// GITHUB_TOKEN is only sent to known GitHub hosts to prevent token leakage
// to untrusted third-party servers via transitive dependencies.
//
// Returns nil auth (anonymous) if no credentials are found — the caller
// should attempt the operation and wrap any auth errors with suggestions.
func Auth(url string) transport.AuthMethod {
	// CRAFT_TOKEN: scoped to trusted hosts
	if token := os.Getenv("CRAFT_TOKEN"); token != "" && isTrustedHost(url) {
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

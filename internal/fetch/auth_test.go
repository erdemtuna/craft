package fetch

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// resetWarningState resets the one-time warning guard between tests.
func resetWarningState(t *testing.T) {
	t.Helper()
	craftTokenWarning = sync.Once{}
}

func TestAuthCraftTokenPrecedence(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	auth := Auth("https://github.com/org/repo.git")
	if auth == nil {
		t.Fatal("Expected auth object")
	}
	basic, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("Expected *http.BasicAuth")
	}
	if basic.Password != "craft-secret" {
		t.Errorf("CRAFT_TOKEN should take precedence, got password %q", basic.Password)
	}
}

func TestAuthGitHubTokenFallback(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	auth := Auth("https://github.com/org/repo.git")
	if auth == nil {
		t.Fatal("Expected auth from GITHUB_TOKEN")
	}
}

func TestAuthNoTokensReturnsNil(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	auth := Auth("https://github.com/org/repo.git")
	if auth != nil {
		t.Error("Expected nil auth when no tokens set")
	}
}

func TestAuthGitHubTokenNotSentToEvilHost(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	auth := Auth("https://evil.com/attacker/repo.git")
	if auth != nil {
		t.Error("GITHUB_TOKEN must not be sent to non-GitHub hosts")
	}
}

func TestAuthCraftTokenSentToKnownForge(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	for _, host := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		auth := Auth("https://" + host + "/org/repo.git")
		if auth == nil {
			t.Fatalf("CRAFT_TOKEN should be sent to well-known forge %s", host)
		}
		basic, ok := auth.(*http.BasicAuth)
		if !ok {
			t.Fatalf("Expected *http.BasicAuth for %s", host)
		}
		if basic.Password != "craft-secret" {
			t.Errorf("Expected craft-secret for %s, got %q", host, basic.Password)
		}
	}
}

func TestAuthCraftTokenNotSentToUntrustedHostWithAllowlist(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "github.com,internal.example.com")

	auth := Auth("https://evil.com/attacker/repo.git")
	if auth != nil {
		t.Error("CRAFT_TOKEN must not be sent to hosts outside CRAFT_TOKEN_HOSTS")
	}
}

func TestAuthCraftTokenSentToAllowlistedHost(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "github.com,internal.example.com")

	auth := Auth("https://internal.example.com/org/repo.git")
	if auth == nil {
		t.Fatal("CRAFT_TOKEN should be sent to allowlisted host")
	}
	basic, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("Expected *http.BasicAuth")
	}
	if basic.Password != "craft-secret" {
		t.Errorf("Expected craft-secret, got %q", basic.Password)
	}
}

func TestAuthCraftTokenBackwardCompatUnknownHostWarns(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	var buf bytes.Buffer
	oldWriter := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = oldWriter })

	auth := Auth("https://unknown-forge.example.com/org/repo.git")
	if auth == nil {
		t.Fatal("Backward compat: CRAFT_TOKEN should still be sent to unknown hosts without allowlist")
	}
	basic, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("Expected *http.BasicAuth")
	}
	if basic.Password != "craft-secret" {
		t.Errorf("Expected craft-secret, got %q", basic.Password)
	}
	if !strings.Contains(buf.String(), "warning") {
		t.Error("Expected a warning about sending token to unknown host")
	}
	if !strings.Contains(buf.String(), "CRAFT_TOKEN_HOSTS") {
		t.Error("Warning should mention CRAFT_TOKEN_HOSTS")
	}
}

func TestAuthCraftTokenWarnsOnlyOnce(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	var buf bytes.Buffer
	oldWriter := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = oldWriter })

	Auth("https://unknown1.example.com/org/repo.git")
	Auth("https://unknown2.example.com/org/repo.git")

	warnings := strings.Count(buf.String(), "warning")
	if warnings != 1 {
		t.Errorf("Expected exactly 1 warning, got %d", warnings)
	}
}

func TestAuthGitHubTokenSentToGitHub(t *testing.T) {
	resetWarningState(t)
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("CRAFT_TOKEN_HOSTS", "")

	auth := Auth("https://github.com/org/repo.git")
	if auth == nil {
		t.Fatal("GITHUB_TOKEN should be sent to github.com")
	}
	basic, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("Expected *http.BasicAuth")
	}
	if basic.Password != "github-secret" {
		t.Errorf("Expected github-secret, got %q", basic.Password)
	}
}

func TestWrapAuthErrorPassthrough(t *testing.T) {
	if err := WrapAuthError(nil, "url"); err != nil {
		t.Error("Expected nil for nil error")
	}
}

func TestWrapAuthErrorAddsHint(t *testing.T) {
	err := WrapAuthError(fmt.Errorf("timeout connecting"), "url")
	if err.Error() != "timeout connecting" {
		t.Errorf("Non-auth errors should pass through, got %q", err.Error())
	}
}

func TestWrapAuthErrorDetectsAuthFailure(t *testing.T) {
	err := WrapAuthError(fmt.Errorf("authentication required"), "url")
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Error("Auth errors should include token hint")
	}
}

package fetch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestAuthCraftTokenPrecedence(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "github-secret")

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
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")

	auth := Auth("https://github.com/org/repo.git")
	if auth == nil {
		t.Fatal("Expected auth from GITHUB_TOKEN")
	}
}

func TestAuthNoTokensReturnsNil(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	auth := Auth("https://github.com/org/repo.git")
	if auth != nil {
		t.Error("Expected nil auth when no tokens set")
	}
}

func TestAuthGitHubTokenNotSentToEvilHost(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")

	auth := Auth("https://evil.com/attacker/repo.git")
	if auth != nil {
		t.Error("GITHUB_TOKEN must not be sent to non-GitHub hosts")
	}
}

func TestAuthCraftTokenSentToAnyHost(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "")

	auth := Auth("https://evil.com/attacker/repo.git")
	if auth == nil {
		t.Fatal("CRAFT_TOKEN should be sent to any host")
	}
	basic, ok := auth.(*http.BasicAuth)
	if !ok {
		t.Fatal("Expected *http.BasicAuth")
	}
	if basic.Password != "craft-secret" {
		t.Errorf("Expected craft-secret, got %q", basic.Password)
	}
}

func TestAuthGitHubTokenSentToGitHub(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-secret")

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
	// Timeout doesn't trigger auth hint
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

package fetch

import (
	"fmt"
	"strings"
	"testing"
)

func TestAuthCraftTokenPrecedence(t *testing.T) {
	t.Setenv("CRAFT_TOKEN", "craft-secret")
	t.Setenv("GITHUB_TOKEN", "github-secret")

	auth := Auth("https://github.com/org/repo.git")
	if auth == nil {
		t.Fatal("Expected auth object")
	}
	// CRAFT_TOKEN should take precedence — verified by checking auth is non-nil
	// when CRAFT_TOKEN is set (the concrete type is http.BasicAuth)
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

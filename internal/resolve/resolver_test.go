package resolve

import (
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
)

func newTestFetcher() *fetch.MockFetcher {
	return fetch.NewMockFetcher()
}

func setupDep(mock *fetch.MockFetcher, identity, version, commitSHA string, skillMD string) {
	cloneURL := "https://" + identity + ".git"
	tag := "v" + version
	mock.Refs[cloneURL+":"+tag] = commitSHA
	mock.Trees[cloneURL+":"+commitSHA] = []string{"skills/s1/SKILL.md"}
	mock.Files[cloneURL+":"+commitSHA+":skills/s1/SKILL.md"] = []byte(skillMD)
}

func TestResolveEmpty(t *testing.T) {
	mock := newTestFetcher()
	resolver := NewResolver(mock)

	m := &manifest.Manifest{Name: "test", Dependencies: nil}
	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 0 {
		t.Errorf("Expected 0 resolved, got %d", len(result.Resolved))
	}
}

func TestResolveSingleDep(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/org/skills", "1.0.0", "abc123", "---\nname: my-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]string{"skills": "github.com/org/skills@v1.0.0"},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	if result.Resolved[0].Commit != "abc123" {
		t.Errorf("Commit = %q, want abc123", result.Resolved[0].Commit)
	}
	if len(result.Resolved[0].Skills) != 1 || result.Resolved[0].Skills[0] != "my-skill" {
		t.Errorf("Skills = %v, want [my-skill]", result.Resolved[0].Skills)
	}
}

func TestResolveMVSDiamond(t *testing.T) {
	mock := newTestFetcher()

	// A depends on C@v1.0.0 via B, and C@v1.2.0 via D
	// Root depends on B and D
	bClone := "https://github.com/org/b.git"
	dClone := "https://github.com/org/d.git"
	cClone := "https://github.com/org/c.git"

	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  c: github.com/org/c@v1.0.0\n")

	mock.Refs[dClone+":v1.0.0"] = "ddd"
	mock.Trees[dClone+":ddd"] = []string{"skills/d-skill/SKILL.md"}
	mock.Files[dClone+":ddd:skills/d-skill/SKILL.md"] = []byte("---\nname: d-skill\n---\n")
	mock.Files[dClone+":ddd:craft.yaml"] = []byte("schema_version: 1\nname: d\nversion: 1.0.0\nskills:\n  - ./skills/d-skill\ndependencies:\n  c: github.com/org/c@v1.2.0\n")

	// C at both versions
	mock.Refs[cClone+":v1.0.0"] = "c100"
	mock.Refs[cClone+":v1.2.0"] = "c120"
	mock.Trees[cClone+":c100"] = []string{"skills/c-skill/SKILL.md"}
	mock.Trees[cClone+":c120"] = []string{"skills/c-skill/SKILL.md"}
	mock.Files[cClone+":c100:skills/c-skill/SKILL.md"] = []byte("---\nname: c-skill\n---\n")
	mock.Files[cClone+":c120:skills/c-skill/SKILL.md"] = []byte("---\nname: c-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "root",
		Dependencies: map[string]string{
			"b": "github.com/org/b@v1.0.0",
			"d": "github.com/org/d@v1.0.0",
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should have B, D, and C (MVS selects C@v1.2.0)
	if len(result.Resolved) != 3 {
		t.Fatalf("Expected 3 resolved deps, got %d", len(result.Resolved))
	}

	// Find C in results — should be v1.2.0 (commit c120)
	for _, dep := range result.Resolved {
		if strings.Contains(dep.URL, "github.com/org/c") {
			if dep.Commit != "c120" {
				t.Errorf("MVS should select C@v1.2.0 (commit c120), got commit %q", dep.Commit)
			}
			if !strings.Contains(dep.URL, "v1.2.0") {
				t.Errorf("MVS should select v1.2.0, got URL %q", dep.URL)
			}
		}
	}
}

func TestResolveCycleDetection(t *testing.T) {
	mock := newTestFetcher()

	aClone := "https://github.com/org/a.git"
	bClone := "https://github.com/org/b.git"

	mock.Refs[aClone+":v1.0.0"] = "aaa"
	mock.Files[aClone+":aaa:craft.yaml"] = []byte("schema_version: 1\nname: a\nversion: 1.0.0\nskills:\n  - ./s\ndependencies:\n  b: github.com/org/b@v1.0.0\n")
	mock.Trees[aClone+":aaa"] = []string{"s/SKILL.md"}
	mock.Files[aClone+":aaa:s/SKILL.md"] = []byte("---\nname: a-skill\n---\n")

	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./s\ndependencies:\n  a: github.com/org/a@v1.0.0\n")
	mock.Trees[bClone+":bbb"] = []string{"s/SKILL.md"}
	mock.Files[bClone+":bbb:s/SKILL.md"] = []byte("---\nname: b-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "root",
		Dependencies: map[string]string{"a": "github.com/org/a@v1.0.0"},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Error should mention cycle, got: %v", err)
	}
}

func TestResolveCollision(t *testing.T) {
	mock := newTestFetcher()

	// Two deps export the same skill name
	setupDep(mock, "github.com/org/a", "1.0.0", "aaa", "---\nname: shared-skill\n---\n")
	setupDep(mock, "github.com/org/b", "1.0.0", "bbb", "---\nname: shared-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "root",
		Dependencies: map[string]string{
			"a": "github.com/org/a@v1.0.0",
			"b": "github.com/org/b@v1.0.0",
		},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected collision error")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("Error should mention collision, got: %v", err)
	}
	if !strings.Contains(err.Error(), "shared-skill") {
		t.Errorf("Error should mention skill name, got: %v", err)
	}
}

func TestResolvePinfileReuse(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/org/skills", "1.0.0", "abc123", "---\nname: my-skill\n---\n")

	existing := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/org/skills@v1.0.0": {
				Commit:    "pinned-commit",
				Integrity: "sha256-pinned=",
				Skills:    []string{"my-skill"},
			},
		},
	}

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]string{"skills": "github.com/org/skills@v1.0.0"},
	}

	result, err := resolver.Resolve(m, ResolveOptions{ExistingPinfile: existing})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should reuse pinned commit, not re-resolve
	if result.Resolved[0].Commit != "pinned-commit" {
		t.Errorf("Should reuse pinned commit, got %q", result.Resolved[0].Commit)
	}
}

func TestResolveForceResolve(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/org/skills", "1.0.0", "fresh-commit", "---\nname: my-skill\n---\n")

	existing := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/org/skills@v1.0.0": {
				Commit:    "old-commit",
				Integrity: "sha256-old=",
				Skills:    []string{"my-skill"},
			},
		},
	}

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]string{"skills": "github.com/org/skills@v1.0.0"},
	}

	result, err := resolver.Resolve(m, ResolveOptions{
		ExistingPinfile: existing,
		ForceResolve:    map[string]bool{"github.com/org/skills@v1.0.0": true},
	})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should use fresh commit, not pinned
	if result.Resolved[0].Commit != "fresh-commit" {
		t.Errorf("Should use fresh commit when forced, got %q", result.Resolved[0].Commit)
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.0", "1.1.9", 1},
		{"0.0.1", "0.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

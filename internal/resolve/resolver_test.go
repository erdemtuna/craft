package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/semver"
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
	if result.Resolved == nil {
		t.Error("Expected non-nil empty slice for Resolved, got nil")
	}
}

func TestResolveSingleDep(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/org/skills", "1.0.0", "abc123", "---\nname: my-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]manifest.DependencySpec{"skills": {URL: "github.com/org/skills@v1.0.0"}},
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
		Dependencies: map[string]manifest.DependencySpec{
			"b": {URL: "github.com/org/b@v1.0.0"},
			"d": {URL: "github.com/org/d@v1.0.0"},
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
		Dependencies: map[string]manifest.DependencySpec{"a": {URL: "github.com/org/a@v1.0.0"}},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Error should mention cycle, got: %v", err)
	}
}

func TestResolveSameNameSkillsAllowed(t *testing.T) {
	mock := newTestFetcher()

	// Two deps export the same skill name — should succeed with namespacing
	setupDep(mock, "github.com/org/a", "1.0.0", "aaa", "---\nname: shared-skill\n---\n")
	setupDep(mock, "github.com/org/b", "1.0.0", "bbb", "---\nname: shared-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "root",
		Dependencies: map[string]manifest.DependencySpec{
			"a": {URL: "github.com/org/a@v1.0.0"},
			"b": {URL: "github.com/org/b@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve should succeed with same-name skills from different deps, got: %v", err)
	}
	if len(result.Resolved) != 2 {
		t.Errorf("Expected 2 resolved deps, got %d", len(result.Resolved))
	}
}

func TestResolvePinfileReuse(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/org/skills", "1.0.0", "abc123", "---\nname: my-skill\n---\n")

	existing := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/org/skills@v1.0.0": {
				Commit:     "pinned-commit",
				Integrity:  "sha256-pinned=",
				Skills:     []string{"my-skill"},
				SkillPaths: []string{"skills/my-skill"},
			},
		},
	}

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]manifest.DependencySpec{"skills": {URL: "github.com/org/skills@v1.0.0"}},
	}

	result, err := resolver.Resolve(m, ResolveOptions{ExistingPinfile: existing})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should reuse pinned commit, not re-resolve
	if result.Resolved[0].Commit != "pinned-commit" {
		t.Errorf("Should reuse pinned commit, got %q", result.Resolved[0].Commit)
	}
	// Should preserve SkillPaths from pinfile
	if len(result.Resolved[0].SkillPaths) != 1 || result.Resolved[0].SkillPaths[0] != "skills/my-skill" {
		t.Errorf("Should preserve SkillPaths from pinfile, got %v", result.Resolved[0].SkillPaths)
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
		Dependencies: map[string]manifest.DependencySpec{"skills": {URL: "github.com/org/skills@v1.0.0"}},
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
			got := semver.Compare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("semver.Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestResolveDepthLimit(t *testing.T) {
	mock := newTestFetcher()

	// Build a chain of maxResolutionDepth+2 packages: dep0 → dep1 → ... → depN
	// Each dep depends on the next, exceeding the depth limit.
	chainLen := maxResolutionDepth + 2
	for i := 0; i < chainLen; i++ {
		identity := fmt.Sprintf("github.com/org/dep%d", i)
		cloneURL := "https://" + identity + ".git"
		commitSHA := fmt.Sprintf("sha%d", i)
		mock.Refs[cloneURL+":v1.0.0"] = commitSHA
		mock.Trees[cloneURL+":"+commitSHA] = []string{"skills/s/SKILL.md"}
		mock.Files[cloneURL+":"+commitSHA+":skills/s/SKILL.md"] = []byte(fmt.Sprintf("---\nname: skill-%d\n---\n", i))

		if i < chainLen-1 {
			nextIdentity := fmt.Sprintf("github.com/org/dep%d", i+1)
			mock.Files[cloneURL+":"+commitSHA+":craft.yaml"] = []byte(fmt.Sprintf(
				"schema_version: 1\nname: dep%d\nversion: 1.0.0\nskills:\n  - ./skills/s\ndependencies:\n  next: %s@v1.0.0\n", i, nextIdentity))
		}
	}

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "root",
		Dependencies: map[string]manifest.DependencySpec{"dep0": {URL: "github.com/org/dep0@v1.0.0"}},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected depth limit error")
	}
	if !strings.Contains(err.Error(), "exceeded maximum depth") {
		t.Errorf("Error should mention depth limit, got: %v", err)
	}
}

func TestResolveMVSTransitiveDepsReCollection(t *testing.T) {
	mock := newTestFetcher()

	aClone := "https://github.com/org/a.git"
	bClone := "https://github.com/org/b.git"
	cClone := "https://github.com/org/c.git"
	dClone := "https://github.com/org/d.git"

	// A@v1.0.0 depends on C@v1.0.0
	mock.Refs[aClone+":v1.0.0"] = "aaa"
	mock.Trees[aClone+":aaa"] = []string{"skills/a-skill/SKILL.md"}
	mock.Files[aClone+":aaa:skills/a-skill/SKILL.md"] = []byte("---\nname: a-skill\n---\n")
	mock.Files[aClone+":aaa:craft.yaml"] = []byte("schema_version: 1\nname: a\nversion: 1.0.0\nskills:\n  - ./skills/a-skill\ndependencies:\n  c: github.com/org/c@v1.0.0\n")

	// B@v1.0.0 depends on C@v2.0.0
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  c: github.com/org/c@v2.0.0\n")

	// C@v1.0.0 has no transitive deps (no craft.yaml)
	mock.Refs[cClone+":v1.0.0"] = "c100"
	mock.Trees[cClone+":c100"] = []string{"skills/c-skill/SKILL.md"}
	mock.Files[cClone+":c100:skills/c-skill/SKILL.md"] = []byte("---\nname: c-skill\n---\n")

	// C@v2.0.0 depends on D@v1.0.0
	mock.Refs[cClone+":v2.0.0"] = "c200"
	mock.Trees[cClone+":c200"] = []string{"skills/c-skill/SKILL.md"}
	mock.Files[cClone+":c200:skills/c-skill/SKILL.md"] = []byte("---\nname: c-skill\n---\n")
	mock.Files[cClone+":c200:craft.yaml"] = []byte("schema_version: 1\nname: c\nversion: 2.0.0\nskills:\n  - ./skills/c-skill\ndependencies:\n  d: github.com/org/d@v1.0.0\n")

	// D@v1.0.0 has no transitive deps
	mock.Refs[dClone+":v1.0.0"] = "ddd"
	mock.Trees[dClone+":ddd"] = []string{"skills/d-skill/SKILL.md"}
	mock.Files[dClone+":ddd:skills/d-skill/SKILL.md"] = []byte("---\nname: d-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "root",
		Dependencies: map[string]manifest.DependencySpec{
			"a": {URL: "github.com/org/a@v1.0.0"},
			"b": {URL: "github.com/org/b@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should have A, B, C (MVS selects v2.0.0), and D
	if len(result.Resolved) != 4 {
		var urls []string
		for _, d := range result.Resolved {
			urls = append(urls, d.URL)
		}
		t.Fatalf("Expected 4 resolved deps, got %d: %v", len(result.Resolved), urls)
	}

	// Verify D@v1.0.0 is in the resolved set
	foundD := false
	for _, dep := range result.Resolved {
		if strings.Contains(dep.URL, "github.com/org/d") {
			foundD = true
			if dep.Commit != "ddd" {
				t.Errorf("D should have commit ddd, got %q", dep.Commit)
			}
			if !strings.Contains(dep.URL, "v1.0.0") {
				t.Errorf("D should be v1.0.0, got URL %q", dep.URL)
			}
		}
	}
	if !foundD {
		t.Error("D@v1.0.0 should be in resolved set (transitive dep of C@v2.0.0)")
	}

	// Verify C is v2.0.0 (MVS selected)
	for _, dep := range result.Resolved {
		if strings.Contains(dep.URL, "github.com/org/c") {
			if dep.Commit != "c200" {
				t.Errorf("MVS should select C@v2.0.0 (commit c200), got commit %q", dep.Commit)
			}
		}
	}
}

func setupBranchDep(mock *fetch.MockFetcher, identity, branch, commitSHA string, skillMD string) {
	cloneURL := "https://" + identity + ".git"
	mock.Refs[cloneURL+":"+branch] = commitSHA
	mock.Trees[cloneURL+":"+commitSHA] = []string{"skills/s1/SKILL.md"}
	mock.Files[cloneURL+":"+commitSHA+":skills/s1/SKILL.md"] = []byte(skillMD)
}

func setupCommitDep(mock *fetch.MockFetcher, identity, commitSHA string, skillMD string) {
	cloneURL := "https://" + identity + ".git"
	mock.Refs[cloneURL+":"+commitSHA] = commitSHA
	mock.Trees[cloneURL+":"+commitSHA] = []string{"skills/s1/SKILL.md"}
	mock.Files[cloneURL+":"+commitSHA+":skills/s1/SKILL.md"] = []byte(skillMD)
}

func TestResolveBranchDep(t *testing.T) {
	mock := newTestFetcher()
	setupBranchDep(mock, "github.com/acme/tools", "main", "branchcommit123", "---\nname: tool-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]manifest.DependencySpec{"tools": {URL: "github.com/acme/tools@branch:main"}},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	dep := result.Resolved[0]
	if dep.Commit != "branchcommit123" {
		t.Errorf("Commit = %q, want branchcommit123", dep.Commit)
	}
	if dep.RefType != RefTypeBranch {
		t.Errorf("RefType = %q, want %q", dep.RefType, RefTypeBranch)
	}
	if len(dep.Skills) != 1 || dep.Skills[0] != "tool-skill" {
		t.Errorf("Skills = %v, want [tool-skill]", dep.Skills)
	}

	// Check pinfile has ref_type
	entry, ok := result.Pinfile.Resolved["github.com/acme/tools@branch:main"]
	if !ok {
		t.Fatal("Pinfile missing entry for branch dep")
	}
	if entry.RefType != "branch" {
		t.Errorf("Pinfile RefType = %q, want %q", entry.RefType, "branch")
	}
}

func TestResolveCommitDep(t *testing.T) {
	mock := newTestFetcher()
	setupCommitDep(mock, "github.com/acme/tools", "abc1234def567890abc1234def567890abc1234d", "---\nname: tool-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]manifest.DependencySpec{"tools": {URL: "github.com/acme/tools@abc1234def567890abc1234def567890abc1234d"}},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	dep := result.Resolved[0]
	if dep.Commit != "abc1234def567890abc1234def567890abc1234d" {
		t.Errorf("Commit = %q, want abc1234def567890abc1234def567890abc1234d", dep.Commit)
	}
	if dep.RefType != RefTypeCommit {
		t.Errorf("RefType = %q, want %q", dep.RefType, RefTypeCommit)
	}

	// Check pinfile has ref_type
	entry, ok := result.Pinfile.Resolved["github.com/acme/tools@abc1234def567890abc1234def567890abc1234d"]
	if !ok {
		t.Fatal("Pinfile missing entry for commit dep")
	}
	if entry.RefType != "commit" {
		t.Errorf("Pinfile RefType = %q, want %q", entry.RefType, "commit")
	}
}

func TestResolveMixedRefTypeConflict(t *testing.T) {
	mock := newTestFetcher()
	setupDep(mock, "github.com/acme/tools", "1.0.0", "tagcommit", "---\nname: tag-skill\n---\n")
	setupBranchDep(mock, "github.com/acme/tools", "main", "branchcommit", "---\nname: branch-skill\n---\n")

	// Direct dep is tagged, transitive from B requires branch
	bClone := "https://github.com/org/b.git"
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools: github.com/acme/tools@branch:main\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@v1.0.0"},
			"b":     {URL: "github.com/org/b@v1.0.0"},
		},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected conflict error for mixed ref types, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting ref types") {
		t.Errorf("Error = %q, want conflict message", err.Error())
	}
}

func TestResolveSameBranchMerge(t *testing.T) {
	mock := newTestFetcher()
	setupBranchDep(mock, "github.com/acme/tools", "main", "branchcommit123", "---\nname: tool-skill\n---\n")

	// Both B and root require tools@branch:main — should succeed
	bClone := "https://github.com/org/b.git"
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools: github.com/acme/tools@branch:main\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@branch:main"},
			"b":     {URL: "github.com/org/b@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	// Should have 3 deps: tools (branch), b (tag), and they should coexist
	if len(result.Resolved) != 2 {
		t.Fatalf("Expected 2 resolved, got %d", len(result.Resolved))
	}
}

func TestResolveDifferentBranchConflict(t *testing.T) {
	mock := newTestFetcher()
	setupBranchDep(mock, "github.com/acme/tools", "main", "maincommit", "---\nname: tool-skill\n---\n")
	// Also set up develop branch
	toolsClone := "https://github.com/acme/tools.git"
	mock.Refs[toolsClone+":develop"] = "devcommit"
	mock.Trees[toolsClone+":devcommit"] = []string{"skills/s1/SKILL.md"}
	mock.Files[toolsClone+":devcommit:skills/s1/SKILL.md"] = []byte("---\nname: tool-skill\n---\n")

	bClone := "https://github.com/org/b.git"
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools: github.com/acme/tools@branch:develop\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@branch:main"},
			"b":     {URL: "github.com/org/b@v1.0.0"},
		},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected conflict error for different branches, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting branch names") {
		t.Errorf("Error = %q, want branch conflict message", err.Error())
	}
}

func TestResolveBranchDepBypassesPinfileCache(t *testing.T) {
	mock := newTestFetcher()
	setupBranchDep(mock, "github.com/acme/tools", "main", "freshcommit123abc", "---\nname: tool-skill\n---\n")

	existingPinfile := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/acme/tools@branch:main": {
				Commit:    "stalecommit999def",
				Integrity: "sha256-stale=",
				RefType:   "branch",
				Skills:    []string{"tool-skill"},
			},
		},
	}

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name:         "test",
		Dependencies: map[string]manifest.DependencySpec{"tools": {URL: "github.com/acme/tools@branch:main"}},
	}

	result, err := resolver.Resolve(m, ResolveOptions{ExistingPinfile: existingPinfile})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	if result.Resolved[0].Commit != "freshcommit123abc" {
		t.Errorf("Branch dep should bypass pinfile cache: got commit %q, want freshcommit123abc", result.Resolved[0].Commit)
	}
}

func TestResolveConflictingCommitSHAs(t *testing.T) {
	mock := newTestFetcher()

	// Root depends on B and C, both of which transitively depend on
	// the same package (tools) but at different commit SHAs.
	bClone := "https://github.com/org/b.git"
	cClone := "https://github.com/org/c.git"

	commitA := "aaaa1234567890aaaa1234567890aaaa1234aaaa"
	commitB := "bbbb1234567890bbbb1234567890bbbb1234bbbb"

	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools: github.com/acme/tools@" + commitA + "\n")

	mock.Refs[cClone+":v1.0.0"] = "ccc"
	mock.Trees[cClone+":ccc"] = []string{"skills/c-skill/SKILL.md"}
	mock.Files[cClone+":ccc:skills/c-skill/SKILL.md"] = []byte("---\nname: c-skill\n---\n")
	mock.Files[cClone+":ccc:craft.yaml"] = []byte("schema_version: 1\nname: c\nversion: 1.0.0\nskills:\n  - ./skills/c-skill\ndependencies:\n  tools: github.com/acme/tools@" + commitB + "\n")

	setupCommitDep(mock, "github.com/acme/tools", commitA, "---\nname: tool-skill\n---\n")
	setupCommitDep(mock, "github.com/acme/tools", commitB, "---\nname: tool-skill\n---\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"b": {URL: "github.com/org/b@v1.0.0"},
			"c": {URL: "github.com/org/c@v1.0.0"},
		},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected conflict error for different commit SHAs, got nil")
	}
	if !strings.Contains(err.Error(), "conflicting commit SHAs") {
		t.Errorf("Error = %q, want commit SHA conflict message", err.Error())
	}
}

// --- Phase 3: Select/Filter tests ---

func TestFilterBySelect(t *testing.T) {
	names := []string{"docx", "pdf", "xlsx"}
	dirs := []string{"skills/docx", "skills/pdf", "skills/xlsx"}
	files := map[string][]byte{
		"skills/docx/SKILL.md":  []byte("docx"),
		"skills/docx/prompt.md": []byte("docx prompt"),
		"skills/pdf/SKILL.md":   []byte("pdf"),
		"skills/xlsx/SKILL.md":  []byte("xlsx"),
	}

	// Select 2 of 3
	n, d, f, err := filterBySelect(names, dirs, files, []string{"skills/docx", "skills/pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(n) != 2 {
		t.Errorf("expected 2 names, got %d", len(n))
	}
	if len(d) != 2 {
		t.Errorf("expected 2 dirs, got %d", len(d))
	}
	if n[0] != "docx" || n[1] != "pdf" {
		t.Errorf("names = %v, want [docx pdf]", n)
	}
	// Verify files only include docx and pdf
	if _, ok := f["skills/xlsx/SKILL.md"]; ok {
		t.Error("filtered files should not include xlsx")
	}
	if _, ok := f["skills/docx/SKILL.md"]; !ok {
		t.Error("filtered files should include docx/SKILL.md")
	}
	if _, ok := f["skills/docx/prompt.md"]; !ok {
		t.Error("filtered files should include docx/prompt.md")
	}

	// Select with ./ prefix normalization
	n2, _, _, err := filterBySelect(names, dirs, files, []string{"./skills/docx"})
	if err != nil {
		t.Fatalf("unexpected error with ./ prefix: %v", err)
	}
	if len(n2) != 1 || n2[0] != "docx" {
		t.Errorf("normalized select: names = %v, want [docx]", n2)
	}

	// Select with trailing / normalization
	n3, _, _, err := filterBySelect(names, dirs, files, []string{"skills/pdf/"})
	if err != nil {
		t.Fatalf("unexpected error with trailing /: %v", err)
	}
	if len(n3) != 1 || n3[0] != "pdf" {
		t.Errorf("normalized select: names = %v, want [pdf]", n3)
	}

	// Select non-existent path → error
	_, _, _, err = filterBySelect(names, dirs, files, []string{"skills/nonexistent"})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
	if err != nil && !strings.Contains(err.Error(), "does not match any skill") {
		t.Errorf("error = %q, want 'does not match' message", err.Error())
	}

	// Empty select = all
	n4, d4, f4, err := filterBySelect(names, dirs, files, nil)
	if err != nil {
		t.Fatalf("unexpected error for nil select: %v", err)
	}
	if len(n4) != 3 {
		t.Errorf("empty select should return all: got %d names", len(n4))
	}
	if len(d4) != 3 {
		t.Errorf("empty select should return all: got %d dirs", len(d4))
	}
	if len(f4) != len(files) {
		t.Errorf("empty select should return all files: got %d, want %d", len(f4), len(files))
	}
}

func setupMultiSkillDep(mock *fetch.MockFetcher, identity, version, commitSHA string) {
	cloneURL := "https://" + identity + ".git"
	tag := "v" + version
	mock.Refs[cloneURL+":"+tag] = commitSHA
	mock.Trees[cloneURL+":"+commitSHA] = []string{
		"skills/docx/SKILL.md",
		"skills/pdf/SKILL.md",
		"skills/xlsx/SKILL.md",
	}
	mock.Files[cloneURL+":"+commitSHA+":skills/docx/SKILL.md"] = []byte("---\nname: docx\n---\n")
	mock.Files[cloneURL+":"+commitSHA+":skills/pdf/SKILL.md"] = []byte("---\nname: pdf\n---\n")
	mock.Files[cloneURL+":"+commitSHA+":skills/xlsx/SKILL.md"] = []byte("---\nname: xlsx\n---\n")
}

func TestResolveEmptySelect(t *testing.T) {
	mock := newTestFetcher()
	setupMultiSkillDep(mock, "github.com/acme/tools", "1.0.0", "abc123")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	dep := result.Resolved[0]
	if len(dep.Skills) != 3 {
		t.Errorf("Empty select should return all skills, got %d: %v", len(dep.Skills), dep.Skills)
	}
	if len(dep.AllSkillPaths) != 3 {
		t.Errorf("AllSkillPaths should have all 3, got %d: %v", len(dep.AllSkillPaths), dep.AllSkillPaths)
	}
}

func TestResolveSelectNotFound(t *testing.T) {
	mock := newTestFetcher()
	setupMultiSkillDep(mock, "github.com/acme/tools", "1.0.0", "abc123")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@v1.0.0", Select: []string{"skills/nonexistent"}},
		},
	}

	_, err := resolver.Resolve(m, ResolveOptions{})
	if err == nil {
		t.Fatal("Expected error for non-existent select path")
	}
	if !strings.Contains(err.Error(), "does not match any skill") {
		t.Errorf("Error = %q, want 'does not match' message", err.Error())
	}
}

func TestResolveSelectMerge(t *testing.T) {
	mock := newTestFetcher()
	setupMultiSkillDep(mock, "github.com/acme/tools", "1.0.0", "abc123")

	// Package B also depends on tools with a different select
	bClone := "https://github.com/org/b.git"
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools: github.com/acme/tools@v1.0.0\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@v1.0.0", Select: []string{"skills/docx"}},
			"b":     {URL: "github.com/org/b@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Find the tools dep — B has empty select, so the merge should result
	// in all skills (empty select wins).
	for _, dep := range result.Resolved {
		if strings.Contains(dep.URL, "acme/tools") {
			if len(dep.Skills) != 3 {
				t.Errorf("Merged select should include all skills (empty select from transitive), got %d: %v", len(dep.Skills), dep.Skills)
			}
			return
		}
	}
	t.Fatal("tools dep not found in resolved")
}

func TestResolveSelectMergeUnion(t *testing.T) {
	mock := newTestFetcher()
	setupMultiSkillDep(mock, "github.com/acme/tools", "1.0.0", "abc123")
	setupMultiSkillDep(mock, "github.com/acme/tools", "2.0.0", "def456")

	// B depends on tools@v2.0.0 with select [skills/xlsx]
	bClone := "https://github.com/org/b.git"
	mock.Refs[bClone+":v1.0.0"] = "bbb"
	mock.Trees[bClone+":bbb"] = []string{"skills/b-skill/SKILL.md"}
	mock.Files[bClone+":bbb:skills/b-skill/SKILL.md"] = []byte("---\nname: b-skill\n---\n")
	mock.Files[bClone+":bbb:craft.yaml"] = []byte("schema_version: 1\nname: b\nversion: 1.0.0\nskills:\n  - ./skills/b-skill\ndependencies:\n  tools:\n    url: github.com/acme/tools@v2.0.0\n    select:\n      - skills/xlsx\n")

	resolver := NewResolver(mock)
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"tools": {URL: "github.com/acme/tools@v1.0.0", Select: []string{"skills/docx"}},
			"b":     {URL: "github.com/org/b@v1.0.0"},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	for _, dep := range result.Resolved {
		if strings.Contains(dep.URL, "acme/tools") {
			// MVS picks v2.0.0 (higher). Merged select should be union of
			// [skills/docx] and [skills/xlsx].
			if len(dep.Skills) != 2 {
				t.Errorf("Merged select should have 2 skills (docx+xlsx), got %d: %v", len(dep.Skills), dep.Skills)
			}
			return
		}
	}
	t.Fatal("tools dep not found in resolved")
}

func TestResolveSelectOverridesExports(t *testing.T) {
	mock := newTestFetcher()

	// Package with craft.yaml that exports only skills/public,
	// but also has skills/internal available via auto-discovery.
	cloneURL := "https://github.com/acme/restricted.git"
	mock.Refs[cloneURL+":v1.0.0"] = "abc123"
	mock.Trees[cloneURL+":abc123"] = []string{
		"craft.yaml",
		"skills/public/SKILL.md",
		"skills/internal/SKILL.md",
	}
	mock.Files[cloneURL+":abc123:craft.yaml"] = []byte("schema_version: 1\nname: restricted\nversion: 1.0.0\nskills:\n  - ./skills/public\n")
	mock.Files[cloneURL+":abc123:skills/public/SKILL.md"] = []byte("---\nname: public-skill\n---\n")
	mock.Files[cloneURL+":abc123:skills/internal/SKILL.md"] = []byte("---\nname: internal-skill\n---\n")

	resolver := NewResolver(mock)

	// Consumer selects skills/internal — which is NOT in the package's exports
	m := &manifest.Manifest{
		Name: "test",
		Dependencies: map[string]manifest.DependencySpec{
			"restricted": {URL: "github.com/acme/restricted@v1.0.0", Select: []string{"skills/internal"}},
		},
	}

	result, err := resolver.Resolve(m, ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if len(result.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	dep := result.Resolved[0]
	if len(dep.Skills) != 1 || dep.Skills[0] != "internal-skill" {
		t.Errorf("Select should override exports, got skills: %v", dep.Skills)
	}
	if len(dep.SkillPaths) != 1 || dep.SkillPaths[0] != "skills/internal" {
		t.Errorf("SkillPaths should be [skills/internal], got: %v", dep.SkillPaths)
	}
	// AllSkillPaths should include both (full auto-discovery)
	if len(dep.AllSkillPaths) != 2 {
		t.Errorf("AllSkillPaths should have 2 (both public and internal), got %d: %v", len(dep.AllSkillPaths), dep.AllSkillPaths)
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/erdemtuna/craft/internal/integrity"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
)

func TestCollectSkillFiles_SingleDep(t *testing.T) {
	mock := fetch.NewMockFetcher()
	cloneURL := "https://github.com/org/repo.git"
	commit := "abc123"

	mock.Trees[cloneURL+":"+commit] = []string{
		"skills/lint/SKILL.md",
		"skills/lint/rules.yaml",
	}
	mock.Files[cloneURL+":"+commit+":skills/lint/SKILL.md"] = []byte("---\nname: lint\n---\n")
	mock.Files[cloneURL+":"+commit+":skills/lint/rules.yaml"] = []byte("rules: []")

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     commit,
				Skills:     []string{"lint"},
				SkillPaths: []string{"skills/lint"},
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	lintFiles, ok := skills["lint"]
	if !ok {
		t.Fatal("expected skill 'lint' in result")
	}

	if len(lintFiles) != 2 {
		t.Errorf("expected 2 files in lint skill, got %d", len(lintFiles))
	}

	if string(lintFiles["SKILL.md"]) != "---\nname: lint\n---\n" {
		t.Errorf("unexpected SKILL.md content: %q", string(lintFiles["SKILL.md"]))
	}
	if string(lintFiles["rules.yaml"]) != "rules: []" {
		t.Errorf("unexpected rules.yaml content: %q", string(lintFiles["rules.yaml"]))
	}
}

func TestCollectSkillFiles_MultipleDeps(t *testing.T) {
	mock := fetch.NewMockFetcher()

	// First dep with two skills
	url1 := "https://github.com/org/skills.git"
	mock.Trees[url1+":commit1"] = []string{
		"skills/lint/SKILL.md",
		"skills/format/SKILL.md",
	}
	mock.Files[url1+":commit1:skills/lint/SKILL.md"] = []byte("lint skill")
	mock.Files[url1+":commit1:skills/format/SKILL.md"] = []byte("format skill")

	// Second dep
	url2 := "https://github.com/org/tools.git"
	mock.Trees[url2+":commit2"] = []string{
		"skills/debug/SKILL.md",
	}
	mock.Files[url2+":commit2:skills/debug/SKILL.md"] = []byte("debug skill")

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/skills@v1.0.0",
				Commit:     "commit1",
				Skills:     []string{"lint", "format"},
				SkillPaths: []string{"skills/lint", "skills/format"},
			},
			{
				URL:        "github.com/org/tools@v2.0.0",
				Commit:     "commit2",
				Skills:     []string{"debug"},
				SkillPaths: []string{"skills/debug"},
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}

	for _, name := range []string{"lint", "format", "debug"} {
		if _, ok := skills[name]; !ok {
			t.Errorf("expected skill %q in result", name)
		}
	}
}

func TestCollectSkillFiles_EmptySkillPath(t *testing.T) {
	mock := fetch.NewMockFetcher()
	cloneURL := "https://github.com/org/single-skill.git"
	commit := "def456"

	mock.Trees[cloneURL+":"+commit] = []string{
		"SKILL.md",
		"config.yaml",
	}
	mock.Files[cloneURL+":"+commit+":SKILL.md"] = []byte("root skill")
	mock.Files[cloneURL+":"+commit+":config.yaml"] = []byte("config: true")

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/single-skill@v1.0.0",
				Commit:     commit,
				Skills:     []string{"single-skill"},
				SkillPaths: []string{""}, // root-level skill
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	files := skills["single-skill"]
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if string(files["SKILL.md"]) != "root skill" {
		t.Errorf("unexpected SKILL.md content: %q", files["SKILL.md"])
	}
}

func TestCollectSkillFiles_SkipsBadDepURL(t *testing.T) {
	mock := fetch.NewMockFetcher()

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:    "not-a-valid-url",
				Commit: "abc123",
				Skills: []string{"phantom"},
			},
		},
	}

	_, err := collectSkillFiles(mock, result)
	if err == nil {
		t.Fatal("expected error for bad dep URL, got nil")
	}
	if !strings.Contains(err.Error(), "collecting files for") {
		t.Errorf("expected 'collecting files for' in error, got: %v", err)
	}
}

func TestCollectSkillFiles_SkipsListTreeFailure(t *testing.T) {
	// MockFetcher returns nil, nil for unknown tree keys, so collectSkillFiles
	// gets a nil allPaths slice and produces no file paths — the skill is
	// effectively skipped since ReadFiles returns an empty map for no paths.
	mock := fetch.NewMockFetcher()
	// Don't populate Trees — ListTree will return nil, nil

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     "abc123",
				Skills:     []string{"missing-tree"},
				SkillPaths: []string{"skills/missing-tree"},
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	// Skill entry may exist but with no files
	if files, ok := skills["missing-tree"]; ok && len(files) > 0 {
		t.Errorf("expected empty files for missing tree, got %d files", len(files))
	}
}

func TestCollectSkillFiles_SkipsReadFilesFailure(t *testing.T) {
	mock := fetch.NewMockFetcher()
	cloneURL := "https://github.com/org/repo.git"
	commit := "abc123"

	// Populate tree but not files
	mock.Trees[cloneURL+":"+commit] = []string{"skills/broken/SKILL.md"}

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     commit,
				Skills:     []string{"broken"},
				SkillPaths: []string{"skills/broken"},
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	// Skill exists but files map should be empty (file key not in mock)
	if files, ok := skills["broken"]; ok && len(files) > 0 {
		t.Errorf("expected empty files for unreadable skill, got %d", len(files))
	}
}

func TestCollectSkillFiles_NoResolvedDeps(t *testing.T) {
	mock := fetch.NewMockFetcher()
	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestCollectSkillFiles_SkillPathsShorterThanSkills(t *testing.T) {
	mock := fetch.NewMockFetcher()
	cloneURL := "https://github.com/org/repo.git"
	commit := "abc123"

	mock.Trees[cloneURL+":"+commit] = []string{"SKILL.md"}
	mock.Files[cloneURL+":"+commit+":SKILL.md"] = []byte("content")

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     commit,
				Skills:     []string{"skill1", "skill2"},
				SkillPaths: []string{""},
				// SkillPaths shorter than Skills — second skill gets empty skillDir
			},
		},
	}

	skills, err := collectSkillFiles(mock, result)
	if err != nil {
		t.Fatalf("collectSkillFiles returned error: %v", err)
	}

	// Both skills should be present
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestWritePinfileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "craft.pin.yaml")

	pf := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/org/repo@v1.0.0": {
				Commit:    "abc123",
				Integrity: "sha256-test",
				Skills:    []string{"lint"},
			},
		},
	}

	if err := writePinfileAtomic(path, pf); err != nil {
		t.Fatalf("writePinfileAtomic error: %v", err)
	}

	// Verify file exists and is valid
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading pinfile: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Fatal("pinfile is empty")
	}

	// Verify temp file was cleaned up
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after atomic write")
	}
}

func TestWritePinfileAtomic_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "craft.pin.yaml")

	// Write initial file
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	pf := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved: map[string]pinfile.ResolvedEntry{
			"github.com/org/repo@v2.0.0": {
				Commit:    "def456",
				Integrity: "sha256-new",
				Skills:    []string{"format"},
			},
		},
	}

	if err := writePinfileAtomic(path, pf); err != nil {
		t.Fatalf("writePinfileAtomic error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) == "old content" {
		t.Error("pinfile should have been overwritten")
	}
}

func TestWritePinfileAtomic_BadPath(t *testing.T) {
	err := writePinfileAtomic("/nonexistent/dir/craft.pin.yaml", &pinfile.Pinfile{
		PinVersion: 1,
		Resolved:   map[string]pinfile.ResolvedEntry{},
	})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestCountSkills(t *testing.T) {
	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{Skills: []string{"a", "b"}},
			{Skills: []string{"c"}},
			{Skills: nil},
		},
	}
	if got := countSkills(result); got != 3 {
		t.Errorf("countSkills = %d, want 3", got)
	}
}

func TestCountSkills_Empty(t *testing.T) {
	result := &resolve.ResolveResult{Resolved: nil}
	if got := countSkills(result); got != 0 {
		t.Errorf("countSkills = %d, want 0", got)
	}
}

func TestResolveInstallTargets_ExplicitPath(t *testing.T) {
	want := "/custom/install/path"
	got, err := resolveInstallTargets(want)
	if err != nil {
		t.Fatalf("resolveInstallTargets error: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("resolveInstallTargets = %v, want [%q]", got, want)
	}
}

func TestVerifyIntegrity_Pass(t *testing.T) {
	// Build skill files matching what the resolver would produce
	skillFiles := map[string]map[string][]byte{
		"lint": {
			"SKILL.md":    []byte("---\nname: lint\n---\n"),
			"rules.yaml":  []byte("rules: []"),
		},
	}

	// Compute the correct digest using original paths (with prefix)
	combined := map[string][]byte{
		"skills/lint/SKILL.md":   []byte("---\nname: lint\n---\n"),
		"skills/lint/rules.yaml": []byte("rules: []"),
	}
	correctDigest := integrity.Digest(combined)

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     "abc123",
				Skills:     []string{"lint"},
				SkillPaths: []string{"skills/lint"},
			},
		},
		Pinfile: &pinfile.Pinfile{
			PinVersion: 1,
			Resolved: map[string]pinfile.ResolvedEntry{
				"github.com/org/repo@v1.0.0": {
					Commit:    "abc123",
					Integrity: correctDigest,
					Skills:    []string{"lint"},
				},
			},
		},
	}

	if err := verifyIntegrity(result, skillFiles); err != nil {
		t.Fatalf("verifyIntegrity returned unexpected error: %v", err)
	}
}

func TestVerifyIntegrity_Mismatch(t *testing.T) {
	// Skill files that don't match the pinfile digest (simulating cache poisoning)
	skillFiles := map[string]map[string][]byte{
		"lint": {
			"SKILL.md": []byte("TAMPERED CONTENT"),
		},
	}

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     "abc123",
				Skills:     []string{"lint"},
				SkillPaths: []string{"skills/lint"},
			},
		},
		Pinfile: &pinfile.Pinfile{
			PinVersion: 1,
			Resolved: map[string]pinfile.ResolvedEntry{
				"github.com/org/repo@v1.0.0": {
					Commit:    "abc123",
					Integrity: "sha256-originaldigest",
					Skills:    []string{"lint"},
				},
			},
		},
	}

	err := verifyIntegrity(result, skillFiles)
	if err == nil {
		t.Fatal("expected integrity mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "integrity mismatch") {
		t.Errorf("expected 'integrity mismatch' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "craft cache clean") {
		t.Errorf("expected 'craft cache clean' hint in error, got: %v", err)
	}
}

func TestVerifyIntegrity_SkipsMissingPinEntry(t *testing.T) {
	skillFiles := map[string]map[string][]byte{
		"lint": {"SKILL.md": []byte("content")},
	}

	result := &resolve.ResolveResult{
		Resolved: []resolve.ResolvedDep{
			{
				URL:        "github.com/org/repo@v1.0.0",
				Commit:     "abc123",
				Skills:     []string{"lint"},
				SkillPaths: []string{"skills/lint"},
			},
		},
		Pinfile: &pinfile.Pinfile{
			PinVersion: 1,
			Resolved:   map[string]pinfile.ResolvedEntry{},
		},
	}

	if err := verifyIntegrity(result, skillFiles); err != nil {
		t.Fatalf("verifyIntegrity should skip deps without pinfile entry, got: %v", err)
	}
}

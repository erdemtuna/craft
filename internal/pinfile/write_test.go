package pinfile

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestWriteRoundTrip(t *testing.T) {
	original := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/git-skills@v1.0.0": {
				Commit:    "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
				Integrity: "sha256-Xk9jR2mN5pQ8vW3yB7cF1dA4hL6tS0uE9iO2wR5nM3s=",
				Skills:    []string{"git-commit", "git-branch"},
			},
			"github.com/other-org/style-skills@v2.3.1": {
				Commit:    "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
				Integrity: "sha256-Lm8kP3qN7rT2wX5yA9bD4eG1hJ6oS0uC3iF2vR5nK7s=",
				Skills:    []string{"python-style", "js-style"},
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(original, &buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.PinVersion != original.PinVersion {
		t.Errorf("PinVersion = %d, want %d", parsed.PinVersion, original.PinVersion)
	}

	if len(parsed.Resolved) != len(original.Resolved) {
		t.Fatalf("Resolved count = %d, want %d", len(parsed.Resolved), len(original.Resolved))
	}

	for url, want := range original.Resolved {
		got, ok := parsed.Resolved[url]
		if !ok {
			t.Errorf("missing entry for %q", url)
			continue
		}
		if got.Commit != want.Commit {
			t.Errorf("Resolved[%q].Commit = %q, want %q", url, got.Commit, want.Commit)
		}
		if got.Integrity != want.Integrity {
			t.Errorf("Resolved[%q].Integrity = %q, want %q", url, got.Integrity, want.Integrity)
		}
		if len(got.Skills) != len(want.Skills) {
			t.Errorf("Resolved[%q].Skills count = %d, want %d", url, len(got.Skills), len(want.Skills))
		}
	}
}

func TestWriteDeterministicOrdering(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"z.com/z/z@v1.0.0": {Commit: "ccc", Integrity: "sha256-c=", Skills: []string{"z"}},
			"a.com/a/a@v1.0.0": {Commit: "aaa", Integrity: "sha256-a=", Skills: []string{"a"}},
			"m.com/m/m@v1.0.0": {Commit: "bbb", Integrity: "sha256-b=", Skills: []string{"m"}},
		},
	}

	var buf1, buf2 bytes.Buffer
	if err := Write(p, &buf1); err != nil {
		t.Fatalf("Write 1 error: %v", err)
	}
	if err := Write(p, &buf2); err != nil {
		t.Fatalf("Write 2 error: %v", err)
	}

	if buf1.String() != buf2.String() {
		t.Error("Two writes of the same Pinfile produced different output")
	}

	output := buf1.String()
	aIdx := strings.Index(output, "a.com/a/a@v1.0.0")
	mIdx := strings.Index(output, "m.com/m/m@v1.0.0")
	zIdx := strings.Index(output, "z.com/z/z@v1.0.0")
	if aIdx > mIdx || mIdx > zIdx {
		t.Error("Resolved entries not sorted alphabetically by URL key")
	}
}

func TestWriteWithSourceField(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/direct/dep@v1.0.0": {
				Commit:    "aaa",
				Integrity: "sha256-a=",
				Skills:    []string{"s1"},
			},
			"github.com/transitive/dep@v2.0.0": {
				Commit:    "bbb",
				Integrity: "sha256-b=",
				Source:     "github.com/direct/dep@v1.0.0",
				Skills:    []string{"s2"},
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(p, &buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "source: github.com/direct/dep@v1.0.0") {
		t.Error("Expected source field for transitive dependency")
	}

	// Verify that direct dep section doesn't have source field.
	// Split output by entry keys: find the direct section and check it.
	directIdx := strings.Index(output, "github.com/direct/dep@v1.0.0")
	transitiveIdx := strings.Index(output, "github.com/transitive/dep@v2.0.0")
	if directIdx < 0 || transitiveIdx < 0 {
		t.Fatal("Expected both entries in output")
	}
	directSection := output[directIdx:transitiveIdx]
	if strings.Contains(directSection, "source:") {
		t.Error("Direct dependency should not have source field")
	}

	// Round-trip
	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Resolved["github.com/transitive/dep@v2.0.0"].Source != "github.com/direct/dep@v1.0.0" {
		t.Error("Source field not preserved after round-trip")
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

func TestWriteError(t *testing.T) {
	p := &Pinfile{PinVersion: 1, Resolved: map[string]ResolvedEntry{}}
	err := Write(p, errWriter{})
	if err == nil {
		t.Fatal("Expected error for failing writer")
	}
}

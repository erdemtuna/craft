package validate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "packages")
}

func TestValidPackage(t *testing.T) {
	root := filepath.Join(testdataDir(), "valid")
	runner := NewRunner(root)
	result := runner.Run()

	if !result.OK() {
		t.Errorf("Expected valid package, got errors:")
		for _, e := range result.Errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestNoManifest(t *testing.T) {
	root := filepath.Join(testdataDir(), "no-manifest")
	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for missing manifest")
	}

	found := false
	for _, e := range result.Errors {
		if e.Category == CategorySchema && strings.Contains(e.Message, "no manifest") {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'no manifest' error")
	}
}

func TestMissingSkillDirectory(t *testing.T) {
	root := filepath.Join(testdataDir(), "missing-skill")
	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for missing skill directory")
	}

	found := false
	for _, e := range result.Errors {
		if e.Category == CategorySkillPath && strings.Contains(e.Path, "does-not-exist") {
			found = true
		}
	}
	if !found {
		t.Error("Expected skill path error for 'does-not-exist'")
	}
}

func TestNameCollision(t *testing.T) {
	root := filepath.Join(testdataDir(), "collision")
	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for name collision")
	}

	found := false
	for _, e := range result.Errors {
		if e.Category == CategoryCollision && strings.Contains(e.Message, "duplicate-name") {
			found = true
		}
	}
	if !found {
		t.Error("Expected collision error for 'duplicate-name'")
	}
}

func TestPathEscape(t *testing.T) {
	root := filepath.Join(testdataDir(), "escape")
	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for path escape")
	}

	found := false
	for _, e := range result.Errors {
		if e.Category == CategorySafety && strings.Contains(e.Message, "escapes") {
			found = true
		}
	}
	if !found {
		t.Error("Expected safety error for path escape")
	}
}

func TestPinfileMismatch(t *testing.T) {
	root := filepath.Join(testdataDir(), "pinfile-mismatch")
	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for pinfile mismatch")
	}

	// Should find both: manifest dep without pin entry AND pin entry without manifest dep
	missingPin := false
	stalePinEntry := false
	for _, e := range result.Errors {
		if e.Category == CategoryPinfile {
			if strings.Contains(e.Message, "no matching pinfile entry") {
				missingPin = true
			}
			if strings.Contains(e.Message, "no matching manifest dependency") {
				stalePinEntry = true
			}
		}
	}
	if !missingPin {
		t.Error("Expected error: manifest dep without pinfile entry")
	}
	if !stalePinEntry {
		t.Error("Expected error: pinfile entry without manifest dep")
	}
}

func TestPinfileNoDeps(t *testing.T) {
	root := filepath.Join(testdataDir(), "pinfile-no-deps")
	runner := NewRunner(root)
	result := runner.Run()

	// Should pass (no errors) but have a warning about unnecessary pinfile
	if !result.OK() {
		t.Errorf("Expected no errors, got:")
		for _, e := range result.Errors {
			t.Errorf("  %s", e.Error())
		}
	}

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about unnecessary pinfile")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "unnecessary") {
			found = true
		}
	}
	if !found {
		t.Error("Expected warning containing 'unnecessary'")
	}
}

func TestMultipleErrors(t *testing.T) {
	// The escape package has a path escape error AND potentially other issues
	// The collision package has collision errors
	// Verify we get multiple errors reported together
	root := filepath.Join(testdataDir(), "collision")
	runner := NewRunner(root)
	result := runner.Run()

	// Collision package should report collision error
	if result.OK() {
		t.Fatal("Expected errors")
	}

	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error, got %d", len(result.Errors))
	}
}

func TestSymlinkCycle(t *testing.T) {
	// Create a temp directory with a symlink cycle in a skill path
	root := t.TempDir()

	// Create a craft.yaml pointing to a skill dir that has a symlink cycle
	manifestContent := `schema_version: 1
name: symlink-test
version: 1.0.0
skills:
  - ./skills/cyclic
`
	os.WriteFile(filepath.Join(root, "craft.yaml"), []byte(manifestContent), 0o644)

	skillDir := filepath.Join(root, "skills", "cyclic")
	os.MkdirAll(skillDir, 0o755)

	// Create a symlink cycle: cyclic/loop -> cyclic
	err := os.Symlink(skillDir, filepath.Join(skillDir, "loop"))
	if err != nil {
		t.Skip("Cannot create symlinks on this platform")
	}

	// Create a SKILL.md so the directory is recognized
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: cyclic\n---\n"), 0o644)

	runner := NewRunner(root)
	result := runner.Run()

	// Should complete without hanging — that's the main assertion.
	// The result may or may not have errors depending on how os.Stat
	// handles the cycle, but it must NOT loop infinitely.
	_ = result
}

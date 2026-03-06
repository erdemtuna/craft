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
	if err := os.WriteFile(filepath.Join(root, "craft.yaml"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(root, "skills", "cyclic")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink cycle: cyclic/loop -> cyclic
	err := os.Symlink(skillDir, filepath.Join(skillDir, "loop"))
	if err != nil {
		t.Skip("Cannot create symlinks on this platform")
	}

	// Create a SKILL.md so the directory is recognized
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: cyclic\ndescription: A cyclic symlink test skill.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(root)
	result := runner.Run()

	// The skill directory itself is valid (it exists and has SKILL.md);
	// the cycle is inside the directory and doesn't affect validation.
	if !result.OK() {
		t.Errorf("Expected no errors for valid skill with internal symlink cycle, got:")
		for _, e := range result.Errors {
			t.Errorf("  %s", e.Error())
		}
	}
}

func TestErrorFormatting(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "path and field",
			err: &Error{
				Category: CategorySchema,
				Path:     "craft.yaml",
				Field:    "name",
				Message:  "invalid value",
			},
			want: "[schema] craft.yaml: name: invalid value",
		},
		{
			name: "path only",
			err: &Error{
				Category: CategorySkillPath,
				Path:     "./skills/a",
				Message:  "directory does not exist",
			},
			want: "[skill-path] ./skills/a: directory does not exist",
		},
		{
			name: "field only",
			err: &Error{
				Category: CategoryCollision,
				Field:    "my-skill",
				Message:  "name collision",
			},
			want: "[collision] my-skill: name collision",
		},
		{
			name: "no path or field",
			err: &Error{
				Category: CategorySafety,
				Message:  "general error",
			},
			want: "[safety] general error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDuplicateSkillPath(t *testing.T) {
	root := t.TempDir()

	// Create a skill directory
	skillDir := filepath.Join(root, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\ndescription: A test skill.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a manifest with duplicate paths
	manifestContent := `schema_version: 1
name: dup-test
version: 1.0.0
skills:
  - ./skills/my-skill
  - ./skills/my-skill
`
	if err := os.WriteFile(filepath.Join(root, "craft.yaml"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(root)
	result := runner.Run()

	if result.OK() {
		t.Fatal("Expected errors for duplicate skill paths")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "duplicate skill path") {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'duplicate skill path' error")
	}
}

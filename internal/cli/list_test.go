package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestListWithDependencies(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
dependencies:
  my-dep: github.com/org/repo@v1.2.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/repo@v1.2.0:
    commit: abc123
    integrity: sha256-test
    skills:
      - skill-a
      - skill-b
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "my-dep") {
		t.Errorf("output should contain alias 'my-dep', got %q", output)
	}
	if !strings.Contains(output, "v1.2.0") {
		t.Errorf("output should contain version 'v1.2.0', got %q", output)
	}
	if !strings.Contains(output, "2 skills") {
		t.Errorf("output should contain '2 skills', got %q", output)
	}
}

func TestListDetailed(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
dependencies:
  my-dep: github.com/org/repo@v1.2.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/repo@v1.2.0:
    commit: abc123
    integrity: sha256-test
    skills:
      - skill-a
      - skill-b
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list", "--detailed"})
	t.Cleanup(func() { listDetailed = false })

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list --detailed failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "github.com/org/repo") {
		t.Errorf("detailed output should contain URL, got %q", output)
	}
	if !strings.Contains(output, "skill-a") {
		t.Errorf("detailed output should contain skill names, got %q", output)
	}
}

func TestListNoPinfile(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("list should error when no pinfile exists")
	}
	if !strings.Contains(err.Error(), "craft.pin.yaml not found") {
		t.Errorf("error should mention missing pinfile, got %q", err.Error())
	}
}

func TestListZeroDependencies(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved: {}
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list with empty deps should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No dependencies resolved") {
		t.Errorf("output should say no dependencies, got %q", output)
	}
}

func TestListSingleSkill(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
dependencies:
  my-dep: github.com/org/repo@v1.0.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/repo@v1.0.0:
    commit: abc123
    integrity: sha256-test
    skills:
      - only-skill
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1 skill)") {
		t.Errorf("output should show '1 skill' (singular), got %q", output)
	}
}

func TestListNoManifest(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	// No craft.yaml at all
	_ = os.Remove("craft.yaml")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("list should error when no manifest exists")
	}
	if !strings.Contains(err.Error(), "craft.yaml not found") {
		t.Errorf("error should mention missing manifest, got %q", err.Error())
	}
}

func TestListSortOrder(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
dependencies:
  zebra: github.com/org/zebra@v1.0.0
  alpha: github.com/org/alpha@v2.0.0
  middle: github.com/org/middle@v1.5.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/zebra@v1.0.0:
    commit: abc
    integrity: sha256-test
    skills:
      - z-skill
  github.com/org/alpha@v2.0.0:
    commit: def
    integrity: sha256-test
    skills:
      - a-skill
  github.com/org/middle@v1.5.0:
    commit: ghi
    integrity: sha256-test
    skills:
      - m-skill
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := buf.String()
	alphaIdx := strings.Index(output, "alpha")
	middleIdx := strings.Index(output, "middle")
	zebraIdx := strings.Index(output, "zebra")

	if alphaIdx == -1 || middleIdx == -1 || zebraIdx == -1 {
		t.Fatalf("output missing deps, got %q", output)
	}
	if !(alphaIdx < middleIdx && middleIdx < zebraIdx) {
		t.Errorf("deps should be sorted alphabetically (alpha < middle < zebra), got %q", output)
	}
}

func TestListDetailedZeroSkills(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
version: 1.0.0
skills:
  - skills/local
dependencies:
  my-dep: github.com/org/repo@v1.0.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/repo@v1.0.0:
    commit: abc123
    integrity: sha256-test
    skills: []
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"list", "--detailed"})
	t.Cleanup(func() { listDetailed = false })

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list --detailed failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "(none)") {
		t.Errorf("detailed output should show '(none)' for zero skills, got %q", output)
	}
}

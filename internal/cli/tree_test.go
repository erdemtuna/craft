package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestTreeWithDependencies(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
skills:
  - skills/local-skill
dependencies:
  my-dep: github.com/org/repo@v1.2.0
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved:
  github.com/org/repo@v1.2.0:
    commit: abc123
    integrity: sha256-test
    skills:
      - remote-skill
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"tree"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("tree command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-pkg") {
		t.Errorf("tree should show package name, got %q", output)
	}
	if !strings.Contains(output, "local-skill") {
		t.Errorf("tree should show local skills, got %q", output)
	}
	if !strings.Contains(output, "my-dep") {
		t.Errorf("tree should show dependency alias, got %q", output)
	}
	if !strings.Contains(output, "remote-skill") {
		t.Errorf("tree should show remote skills, got %q", output)
	}
}

func TestTreeNoDependencies(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
skills:
  - skills/local-skill
`))

	testWriteFile(t, "craft.pin.yaml", []byte(`pin_version: 1
resolved: {}
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"tree"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("tree command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-pkg") {
		t.Errorf("tree should show package name, got %q", output)
	}
	if !strings.Contains(output, "local-skill") {
		t.Errorf("tree should show local skills, got %q", output)
	}
}

func TestTreeNoPinfile(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
skills:
  - skills/local
`))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"tree"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("tree should error when no pinfile exists")
	}
	if !strings.Contains(err.Error(), "craft.pin.yaml not found") {
		t.Errorf("error should mention missing pinfile, got %q", err.Error())
	}
}

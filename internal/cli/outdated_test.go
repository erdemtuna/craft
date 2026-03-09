package cli

import (
"bytes"
"strings"
"testing"

"github.com/erdemtuna/craft/internal/resolve"
)

func TestClassifyUpdate(t *testing.T) {
tests := []struct {
current string
latest  string
want    string
}{
{"1.0.0", "2.0.0", "major"},
{"1.0.0", "1.1.0", "minor"},
{"1.0.0", "1.0.1", "patch"},
{"1.2.3", "2.0.0", "major"},
{"1.2.3", "1.3.0", "minor"},
{"1.2.3", "1.2.4", "patch"},
{"0.1.0", "1.0.0", "major"},
{"0.0.1", "0.0.2", "patch"},
{"0.0.1", "0.1.0", "minor"},
}

for _, tt := range tests {
t.Run(tt.current+"→"+tt.latest, func(t *testing.T) {
got := classifyUpdate(tt.current, tt.latest)
if got != tt.want {
t.Errorf("classifyUpdate(%q, %q) = %q, want %q", tt.current, tt.latest, got, tt.want)
}
})
}
}

func TestSilentExitError(t *testing.T) {
err := &silentExitError{code: 1}
if err.Error() != "exit status 1" {
t.Errorf("silentExitError.Error() = %q, want %q", err.Error(), "exit status 1")
}
}

func TestOutdatedZeroDependencies(t *testing.T) {
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
rootCmd.SetArgs([]string{"outdated"})

err := rootCmd.Execute()
if err != nil {
t.Fatalf("outdated with zero deps should not error: %v", err)
}
if !strings.Contains(buf.String(), "No dependencies to check") {
t.Errorf("expected 'No dependencies to check', got %q", buf.String())
}
}

func TestOutdatedNoPinfile(t *testing.T) {
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

buf := new(bytes.Buffer)
rootCmd.SetOut(buf)
rootCmd.SetErr(new(bytes.Buffer))
rootCmd.SetArgs([]string{"outdated"})

err := rootCmd.Execute()
if err == nil {
t.Fatal("outdated should error when no pinfile exists")
}
if !strings.Contains(err.Error(), "craft.pin.yaml not found") {
t.Errorf("error should mention missing pinfile, got %q", err.Error())
}
}

func TestPrintDryRunSummary(t *testing.T) {
result := &resolve.ResolveResult{
Resolved: []resolve.ResolvedDep{
{
Alias:  "my-dep",
URL:    "github.com/org/repo@v1.0.0",
Skills: []string{"skill-a", "skill-b"},
},
},
}

buf := new(bytes.Buffer)
rootCmd.SetOut(buf)
rootCmd.SetArgs([]string{"version"})

printDryRunSummary(rootCmd, result, "+")

out := buf.String()
if !strings.Contains(out, "Would resolve 1 dependency") {
t.Errorf("dry-run summary should show dependency count, got %q", out)
}
if !strings.Contains(out, "my-dep") {
t.Errorf("dry-run summary should show alias, got %q", out)
}
if !strings.Contains(out, "No changes made") {
t.Errorf("dry-run summary should say 'No changes made', got %q", out)
}
}

func TestInstallDryRunZeroDeps(t *testing.T) {
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
rootCmd.SetArgs([]string{"install", "--dry-run"})
t.Cleanup(func() { installDryRun = false })

err := rootCmd.Execute()
if err != nil {
t.Fatalf("install --dry-run with zero deps should not error: %v", err)
}
if !strings.Contains(buf.String(), "No dependencies to install") {
t.Errorf("expected 'No dependencies to install', got %q", buf.String())
}
}

func TestUpdateDryRunZeroDeps(t *testing.T) {
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
rootCmd.SetArgs([]string{"update", "--dry-run"})
t.Cleanup(func() { updateDryRun = false })

err := rootCmd.Execute()
if err != nil {
t.Fatalf("update --dry-run with zero deps should not error: %v", err)
}
if !strings.Contains(buf.String(), "No dependencies to update") {
t.Errorf("expected 'No dependencies to update', got %q", buf.String())
}
}

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestVerboseFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version", "--verbose"})
	t.Cleanup(func() { verbose = false })

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version --verbose should not error: %v", err)
	}
}

func TestVerboseShorthand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version", "-v"})
	t.Cleanup(func() { verbose = false })

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version -v should not error: %v", err)
	}
}

func TestVerboseLog_Enabled(t *testing.T) {
	oldVerbose := verbose
	defer func() { verbose = oldVerbose }()
	verbose = true

	buf := new(bytes.Buffer)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	// Use verboseLog directly
	verboseLog(rootCmd, "test message %d", 42)

	output := buf.String()
	if !strings.Contains(output, "test message 42") {
		t.Errorf("verbose log should contain message, got %q", output)
	}
}

func TestVerboseLog_Disabled(t *testing.T) {
	oldVerbose := verbose
	defer func() { verbose = oldVerbose }()
	verbose = false

	buf := new(bytes.Buffer)
	rootCmd.SetErr(buf)

	verboseLog(rootCmd, "should not appear")

	output := buf.String()
	if output != "" {
		t.Errorf("verbose log should be empty when disabled, got %q", output)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "hello-world", "hello-world"},
		{"with tab", "hello\tworld", "hello\tworld"},
		{"with newline", "hello\nworld", "hello\nworld"},
		{"with escape", "hello\x1b[31mworld\x1b[0m", "hello[31mworld[0m"},
		{"with null", "hello\x00world", "helloworld"},
		{"with bell", "hello\x07world", "helloworld"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVerboseListOutput(t *testing.T) {
	dir := t.TempDir()
	testChdir(t, dir)

	testWriteFile(t, "craft.yaml", []byte(`schema_version: 1
name: test-pkg
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
`))

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"list", "--verbose"})
	t.Cleanup(func() { verbose = false })

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("list --verbose failed: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "Loaded manifest") {
		t.Errorf("verbose list should emit diagnostic to stderr, got %q", errOutput)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"silent exit 1", &silentExitError{code: 1}, 1},
		{"silent exit 2", &silentExitError{code: 2}, 2},
		{"regular error", fmt.Errorf("some error"), 1},
		{"nil error", nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

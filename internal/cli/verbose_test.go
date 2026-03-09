package cli

import (
	"bytes"
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

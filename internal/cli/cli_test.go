package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/version"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, version.Version) {
		t.Errorf("version output should contain %q, got %q", version.Version, output)
	}
	if !strings.Contains(output, "craft version") {
		t.Errorf("version output should contain 'craft version', got %q", output)
	}
}

func TestRootHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := buf.String()
	for _, sub := range []string{"init", "validate", "version", "list", "tree", "outdated"} {
		if !strings.Contains(output, sub) {
			t.Errorf("help output should list %q subcommand, got %q", sub, output)
		}
	}
}

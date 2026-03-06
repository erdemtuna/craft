package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectClaudeCode(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	result, err := Detect(home)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if result.Agent != ClaudeCode {
		t.Errorf("Agent = %v, want ClaudeCode", result.Agent)
	}
	if result.InstallPath != filepath.Join(home, ".claude", "skills") {
		t.Errorf("InstallPath = %q, want %q", result.InstallPath, filepath.Join(home, ".claude", "skills"))
	}
}

func TestDetectCopilot(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".copilot"), 0o755)

	result, err := Detect(home)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if result.Agent != Copilot {
		t.Errorf("Agent = %v, want Copilot", result.Agent)
	}
	if result.InstallPath != filepath.Join(home, ".copilot", "skills") {
		t.Errorf("InstallPath = %q, want %q", result.InstallPath, filepath.Join(home, ".copilot", "skills"))
	}
}

func TestDetectMultipleAgentsError(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	_ = os.MkdirAll(filepath.Join(home, ".copilot"), 0o755)

	_, err := Detect(home)
	if err == nil {
		t.Fatal("Expected error when multiple agents detected")
	}
	if !strings.Contains(err.Error(), "multiple AI agents detected") {
		t.Errorf("Expected 'multiple AI agents' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--target") {
		t.Errorf("Expected '--target' suggestion in error, got: %v", err)
	}
}

func TestDetectNoAgent(t *testing.T) {
	home := t.TempDir()

	_, err := Detect(home)
	if err == nil {
		t.Fatal("Expected error when no agent detected")
	}
}

func TestDetectFileNotDir(t *testing.T) {
	home := t.TempDir()
	// Create .claude as a file, not a directory
	_ = os.WriteFile(filepath.Join(home, ".claude"), []byte("not a dir"), 0o644)

	_, err := Detect(home)
	if err == nil {
		t.Fatal("Expected error when .claude is a file, not directory")
	}
}

func TestAgentTypeString(t *testing.T) {
	tests := []struct {
		agent Type
		want  string
	}{
		{ClaudeCode, "Claude Code"},
		{Copilot, "GitHub Copilot"},
		{Unknown, "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.agent.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.agent, got, tt.want)
		}
	}
}

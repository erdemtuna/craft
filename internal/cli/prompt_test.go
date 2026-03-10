package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/erdemtuna/craft/internal/agent"
)

func testAgents() []agent.DetectResult {
	return []agent.DetectResult{
		{Agent: agent.ClaudeCode, InstallPath: "/tmp/claude/skills"},
		{Agent: agent.Copilot, InstallPath: "/tmp/copilot/skills"},
	}
}

func TestPromptAgentChoice_ValidFirstAttempt(t *testing.T) {
	agents := testAgents()
	in := strings.NewReader("1\n")
	var errBuf bytes.Buffer

	paths, err := promptAgentChoice(agents, in, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/tmp/claude/skills" {
		t.Errorf("expected Claude path, got: %v", paths)
	}
}

func TestPromptAgentChoice_BothSelection(t *testing.T) {
	agents := testAgents()
	in := strings.NewReader("3\n")
	var errBuf bytes.Buffer

	paths, err := promptAgentChoice(agents, in, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths for Both, got: %v", paths)
	}
}

func TestPromptAgentChoice_RetryOnInvalidThenValid(t *testing.T) {
	agents := testAgents()
	// First line empty (invalid), second line "2" (valid)
	in := strings.NewReader("\n2\n")
	var errBuf bytes.Buffer

	paths, err := promptAgentChoice(agents, in, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/tmp/copilot/skills" {
		t.Errorf("expected Copilot path, got: %v", paths)
	}
	if !strings.Contains(errBuf.String(), "invalid choice") {
		t.Error("expected invalid choice message in stderr")
	}
}

func TestPromptAgentChoice_RetryOnGarbageThenValid(t *testing.T) {
	agents := testAgents()
	// "abc" invalid, "99" out of range, "1" valid
	in := strings.NewReader("abc\n99\n1\n")
	var errBuf bytes.Buffer

	paths, err := promptAgentChoice(agents, in, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/tmp/claude/skills" {
		t.Errorf("expected Claude path, got: %v", paths)
	}
	// Should have two "invalid choice" messages
	count := strings.Count(errBuf.String(), "invalid choice")
	if count != 2 {
		t.Errorf("expected 2 invalid choice messages, got %d", count)
	}
}

func TestPromptAgentChoice_ExhaustedRetries(t *testing.T) {
	agents := testAgents()
	in := strings.NewReader("x\ny\nz\n")
	var errBuf bytes.Buffer

	_, err := promptAgentChoice(agents, in, &errBuf)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if !strings.Contains(err.Error(), "no valid choice") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPromptAgentChoice_NoInput(t *testing.T) {
	agents := testAgents()
	in := strings.NewReader("")
	var errBuf bytes.Buffer

	_, err := promptAgentChoice(agents, in, &errBuf)
	if err == nil {
		t.Fatal("expected error on EOF")
	}
	if !strings.Contains(err.Error(), "no input received") {
		t.Errorf("unexpected error: %v", err)
	}
}

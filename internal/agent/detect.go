// Package agent detects the user's AI agent and determines the
// default skill installation path.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// Type represents a supported AI agent.
type Type int

const (
	// Unknown means no known agent was detected.
	Unknown Type = iota
	// ClaudeCode is Anthropic's Claude Code agent.
	ClaudeCode
	// Copilot is GitHub Copilot.
	Copilot
)

// String returns the human-readable agent name.
func (t Type) String() string {
	switch t {
	case ClaudeCode:
		return "Claude Code"
	case Copilot:
		return "GitHub Copilot"
	default:
		return "Unknown"
	}
}

// DetectResult holds the agent detection outcome.
type DetectResult struct {
	// Agent is the detected agent type.
	Agent Type

	// InstallPath is the default skill installation directory.
	InstallPath string
}

// Detect identifies the user's AI agent by checking for known directory
// markers under the given home directory. Precedence: Claude Code first,
// then GitHub Copilot.
//
// Returns an error if no known agent is detected, suggesting --target.
func Detect(homeDir string) (*DetectResult, error) {
	checks := []struct {
		agent   Type
		marker  string
		install string
	}{
		{ClaudeCode, ".claude", filepath.Join(homeDir, ".claude", "skills")},
		{Copilot, ".copilot", filepath.Join(homeDir, ".copilot", "skills")},
	}

	for _, c := range checks {
		markerPath := filepath.Join(homeDir, c.marker)
		if info, err := os.Stat(markerPath); err == nil && info.IsDir() {
			return &DetectResult{
				Agent:       c.agent,
				InstallPath: c.install,
			}, nil
		}
	}

	return nil, fmt.Errorf("no known AI agent detected — use --target <path> to specify the installation directory")
}

package cli

import (
	"testing"
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

package pinfile

import (
	"strings"
	"testing"
)

func TestValidateValid(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Commit:    "abc123",
				Integrity: "sha256-xyz",
				Skills:    []string{"skill-a"},
			},
		},
	}
	errs := Validate(p)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}
}

func TestValidateEmptySkills(t *testing.T) {
	// Empty skills slice (not nil) is valid — dependency may have no skills
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Commit:    "abc123",
				Integrity: "sha256-xyz",
				Skills:    []string{},
			},
		},
	}
	errs := Validate(p)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for empty skills slice, got %v", errs)
	}
}

func TestValidatePinVersion(t *testing.T) {
	p := &Pinfile{
		PinVersion: 2,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Commit:    "abc123",
				Integrity: "sha256-xyz",
				Skills:    []string{},
			},
		},
	}
	errs := Validate(p)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "pin_version") {
		t.Errorf("Error should mention pin_version, got: %v", errs[0])
	}
}

func TestValidateMissingCommit(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Integrity: "sha256-xyz",
				Skills:    []string{},
			},
		},
	}
	errs := Validate(p)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "commit") {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected commit error, got %v", errs)
	}
}

func TestValidateMissingIntegrity(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Commit: "abc123",
				Skills: []string{},
			},
		},
	}
	errs := Validate(p)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "integrity") {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected integrity error, got %v", errs)
	}
}

func TestValidateMissingSkills(t *testing.T) {
	p := &Pinfile{
		PinVersion: 1,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				Commit:    "abc123",
				Integrity: "sha256-xyz",
				// Skills is nil (not provided)
			},
		},
	}
	errs := Validate(p)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "skills") {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected skills error, got %v", errs)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	p := &Pinfile{
		PinVersion: 2,
		Resolved: map[string]ResolvedEntry{
			"github.com/example/repo@v1.0.0": {
				// All fields missing
			},
		},
	}
	errs := Validate(p)
	// Should have: pin_version, commit, integrity, skills = 4 errors
	if len(errs) < 4 {
		t.Errorf("Expected at least 4 errors, got %d: %v", len(errs), errs)
	}
}

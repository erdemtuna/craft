package pinfile

import "fmt"

// Validate checks a parsed Pinfile against structural rules.
// Returns a slice of all validation errors found.
// Covers the structural half of FR-005; consistency with manifest
// dependencies is checked separately in the validate command (Phase 3).
func Validate(p *Pinfile) []error {
	var errs []error

	// pin_version must be 1
	if p.PinVersion != 1 {
		errs = append(errs, fmt.Errorf("pin_version: must be 1, got %d", p.PinVersion))
	}

	// validate each resolved entry
	for url, entry := range p.Resolved {
		if entry.Commit == "" {
			errs = append(errs, fmt.Errorf("resolved[%q].commit: required field is missing", url))
		}
		if entry.Integrity == "" {
			errs = append(errs, fmt.Errorf("resolved[%q].integrity: required field is missing", url))
		}
		if entry.Skills == nil {
			errs = append(errs, fmt.Errorf("resolved[%q].skills: required field is missing", url))
		}
	}

	return errs
}

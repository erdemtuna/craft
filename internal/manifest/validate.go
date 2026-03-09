package manifest

import (
	"fmt"
	"regexp"
)

// namePattern matches valid package names: lowercase letter followed by
// lowercase alphanumeric segments separated by single hyphens.
// Examples: "my-package", "craft", "code-quality-tools"
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// semverPattern matches strict MAJOR.MINOR.PATCH version strings.
// No pre-release, build metadata, or leading zeros allowed.
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// depURLPattern matches dependency URL format: host/org/repo@<ref>
// where ref is one of: vMAJOR.MINOR.PATCH (tag), hex≥7 (commit SHA), or branch:<name>.
var depURLPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+@(v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)|[0-9a-fA-F]{7,}|branch:.+)$`)

// Validate checks a parsed Manifest against all schema rules.
// Returns a slice of all validation errors found (does not stop at first error).
func Validate(m *Manifest) []error {
	var errs []error

	// schema_version must be 1
	if m.SchemaVersion != 1 {
		errs = append(errs, fmt.Errorf("schema_version: must be 1, got %d", m.SchemaVersion))
	}

	// name is required and must match naming convention
	if m.Name == "" {
		errs = append(errs, fmt.Errorf("name: required field is missing"))
	} else if len(m.Name) > 128 {
		errs = append(errs, fmt.Errorf("name: must be 1–128 characters, got %d", len(m.Name)))
	} else if !namePattern.MatchString(m.Name) {
		errs = append(errs, fmt.Errorf("name: %q does not match required format (lowercase alphanumeric with hyphens, e.g. 'my-package')", m.Name))
	}

	// version is required and must be strict semver
	if m.Version == "" {
		errs = append(errs, fmt.Errorf("version: required field is missing"))
	} else if !semverPattern.MatchString(m.Version) {
		errs = append(errs, fmt.Errorf("version: %q is not valid semver (expected MAJOR.MINOR.PATCH, e.g. '1.0.0')", m.Version))
	}

	// skills must be non-empty
	if len(m.Skills) == 0 {
		errs = append(errs, fmt.Errorf("skills: must contain at least one skill path"))
	}

	// validate dependency URL format for each entry
	for alias, url := range m.Dependencies {
		if !depURLPattern.MatchString(url) {
			errs = append(errs, fmt.Errorf("dependencies[%q]: %q does not match required format (host/org/repo@<ref> where ref is vX.Y.Z, commit SHA, or branch:<name>)", alias, url))
		}
	}

	return errs
}

// ValidateName checks if a string is a valid package/skill name.
// Returns an error describing the issue, or nil if valid.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 128 {
		return fmt.Errorf("name must be 1–128 characters, got %d", len(name))
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%q does not match required format (lowercase alphanumeric with hyphens, e.g. 'my-package')", name)
	}
	return nil
}

// ValidateVersion checks if a string is a valid semver version.
// Returns an error describing the issue, or nil if valid.
func ValidateVersion(ver string) error {
	if ver == "" {
		return fmt.Errorf("version is required")
	}
	if !semverPattern.MatchString(ver) {
		return fmt.Errorf("%q is not valid semver (expected MAJOR.MINOR.PATCH, e.g. '1.0.0')", ver)
	}
	return nil
}

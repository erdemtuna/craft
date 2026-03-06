package skill

import (
	"fmt"

	"github.com/erdemtuna/craft/internal/manifest"
)

// ValidateFrontmatter checks parsed frontmatter against required field rules.
// Returns a slice of all validation errors found.
func ValidateFrontmatter(fm *Frontmatter) []error {
	var errs []error

	// name is required
	if fm.Name == "" {
		errs = append(errs, fmt.Errorf("name: required field is missing"))
		return errs
	}

	// name must follow naming convention (same as package names)
	if err := manifest.ValidateName(fm.Name); err != nil {
		errs = append(errs, fmt.Errorf("name: %w", err))
	}

	return errs
}

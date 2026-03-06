// Package validate orchestrates all craft package validation checks.
// It collects errors across multiple checks and reports them together.
package validate

import "fmt"

// Category classifies the type of validation error.
type Category string

const (
	CategorySchema     Category = "schema"
	CategorySkillPath  Category = "skill-path"
	CategoryFrontmatter Category = "frontmatter"
	CategoryDependency Category = "dependency"
	CategoryPinfile    Category = "pinfile"
	CategoryCollision  Category = "collision"
	CategorySafety     Category = "safety"
)

// Error represents a structured validation error with context for
// actionable user-facing messages.
type Error struct {
	Category   Category
	Path       string // file or directory path relevant to the error
	Field      string // specific field name, if applicable
	Message    string // human-readable error description
	Suggestion string // guidance on how to fix
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Path != "" && e.Field != "" {
		return fmt.Sprintf("[%s] %s: %s: %s", e.Category, e.Path, e.Field, e.Message)
	}
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Category, e.Path, e.Message)
	}
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Category, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Category, e.Message)
}

// Warning represents a non-blocking informational message.
type Warning struct {
	Message string
}

// Result holds the complete output of a validation run.
type Result struct {
	Errors   []*Error
	Warnings []*Warning
}

// OK returns true if no errors were found.
func (r *Result) OK() bool {
	return len(r.Errors) == 0
}

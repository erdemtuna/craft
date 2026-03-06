package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/skill"
)

// Runner orchestrates all validation checks for a craft package.
type Runner struct {
	// Root is the absolute path to the package root directory.
	Root string
}

// NewRunner creates a Runner for the given package root directory.
func NewRunner(root string) *Runner {
	return &Runner{Root: root}
}

// Run executes all validation checks and returns a Result containing
// all errors and warnings found. It does not stop at the first error.
func (r *Runner) Run() *Result {
	result := &Result{}

	// Check 1: Parse and validate craft.yaml
	m := r.checkManifest(result)
	if m == nil {
		// Can't proceed without a valid manifest
		return result
	}

	// Check 2: Validate skill paths and frontmatter
	skillNames := r.checkSkills(result, m)

	// Check 3: Dependency URL validation (already done in manifest.Validate,
	// but we re-check here to add structured errors with paths)
	// This is handled in checkManifest via manifest.Validate

	// Check 4: Pinfile validation and consistency
	r.checkPinfile(result, m)

	// Check 5: Name collision detection
	r.checkNameCollisions(result, skillNames)

	// Check 6: Skill path safety (done inline in checkSkills)
	// Check 7: Symlink cycle detection (done inline in checkSkills)

	return result
}

// checkManifest parses and validates craft.yaml.
// Returns the parsed manifest, or nil if parsing failed.
func (r *Runner) checkManifest(result *Result) *manifest.Manifest {
	manifestPath := filepath.Join(r.Root, "craft.yaml")

	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySchema,
				Path:       "craft.yaml",
				Message:    "no manifest file found",
				Suggestion: "Run 'craft init' to create a craft.yaml manifest",
			})
		} else {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySchema,
				Path:       "craft.yaml",
				Message:    fmt.Sprintf("failed to parse: %v", err),
				Suggestion: "Check craft.yaml for YAML syntax errors",
			})
		}
		return nil
	}

	// Run schema validation
	for _, verr := range manifest.Validate(m) {
		result.Errors = append(result.Errors, &Error{
			Category:   CategorySchema,
			Path:       "craft.yaml",
			Message:    verr.Error(),
			Suggestion: "See craft.yaml format reference for valid field values",
		})
	}

	return m
}

// checkSkills validates each skill path declared in the manifest.
// Returns a map of skill name → list of paths that export that name.
func (r *Runner) checkSkills(result *Result, m *manifest.Manifest) map[string][]string {
	skillNames := make(map[string][]string)
	seen := make(map[string]bool)

	for _, skillPath := range m.Skills {
		cleaned := filepath.Clean(skillPath)
		if seen[cleaned] {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySchema,
				Path:       skillPath,
				Message:    "duplicate skill path in manifest",
				Suggestion: "Remove the duplicate entry from skills[]",
			})
			continue
		}
		seen[cleaned] = true

		// Check 6: Skill path safety — must be relative, within package root
		if filepath.IsAbs(skillPath) {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySafety,
				Path:       skillPath,
				Message:    "skill path must be relative, not absolute",
				Suggestion: "Use a relative path like './skills/my-skill'",
			})
			continue
		}

		// Check for path traversal
		if strings.HasPrefix(cleaned, "..") {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySafety,
				Path:       skillPath,
				Message:    "skill path escapes the package root directory",
				Suggestion: "Skill paths must be within the package root (no '../' escapes)",
			})
			continue
		}

		absPath := filepath.Join(r.Root, cleaned)

		// Check 7: Verify directory exists (also catches symlink cycles
		// since os.Stat follows symlinks and will error on cycles)
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Errors = append(result.Errors, &Error{
					Category:   CategorySkillPath,
					Path:       skillPath,
					Message:    "directory does not exist",
					Suggestion: "Create the directory or remove this path from skills[]",
				})
			} else {
				result.Errors = append(result.Errors, &Error{
					Category:   CategorySkillPath,
					Path:       skillPath,
					Message:    fmt.Sprintf("cannot access: %v", err),
					Suggestion: "Check filesystem permissions and symlink targets",
				})
			}
			continue
		}

		if !info.IsDir() {
			result.Errors = append(result.Errors, &Error{
				Category:   CategorySkillPath,
				Path:       skillPath,
				Message:    "path is not a directory",
				Suggestion: "Skill paths must point to directories containing SKILL.md",
			})
			continue
		}

		// Check for SKILL.md
		skillMDPath := filepath.Join(absPath, "SKILL.md")
		if _, err := os.Stat(skillMDPath); err != nil {
			if os.IsNotExist(err) {
				result.Errors = append(result.Errors, &Error{
					Category:   CategorySkillPath,
					Path:       skillPath,
					Message:    "directory does not contain a SKILL.md file",
					Suggestion: "Add a SKILL.md file with YAML frontmatter to this directory",
				})
			} else {
				result.Errors = append(result.Errors, &Error{
					Category:   CategorySkillPath,
					Path:       filepath.Join(skillPath, "SKILL.md"),
					Message:    fmt.Sprintf("cannot access SKILL.md: %v", err),
					Suggestion: "Check filesystem permissions for this file",
				})
			}
			continue
		}

		// Parse and validate frontmatter
		fm, err := skill.ParseFrontmatterFile(skillMDPath)
		if err != nil {
			result.Errors = append(result.Errors, &Error{
				Category:   CategoryFrontmatter,
				Path:       filepath.Join(skillPath, "SKILL.md"),
				Message:    fmt.Sprintf("frontmatter error: %v", err),
				Suggestion: "Ensure SKILL.md starts with '---' delimiters containing valid YAML",
			})
			continue
		}

		for _, verr := range skill.ValidateFrontmatter(fm) {
			result.Errors = append(result.Errors, &Error{
				Category:   CategoryFrontmatter,
				Path:       filepath.Join(skillPath, "SKILL.md"),
				Message:    verr.Error(),
				Suggestion: "Fix the frontmatter field value",
			})
		}

		// Track name for collision detection
		if fm.Name != "" {
			skillNames[fm.Name] = append(skillNames[fm.Name], skillPath)
		}
	}

	return skillNames
}

// checkPinfile validates the pinfile if it exists and checks consistency
// with the manifest's dependencies.
func (r *Runner) checkPinfile(result *Result, m *manifest.Manifest) {
	pinfilePath := filepath.Join(r.Root, "craft.pin.yaml")

	_, err := os.Stat(pinfilePath)
	if os.IsNotExist(err) {
		// Pinfile is optional — no error
		return
	}

	p, err := pinfile.ParseFile(pinfilePath)
	if err != nil {
		result.Errors = append(result.Errors, &Error{
			Category:   CategoryPinfile,
			Path:       "craft.pin.yaml",
			Message:    fmt.Sprintf("failed to parse: %v", err),
			Suggestion: "Check craft.pin.yaml for YAML syntax errors",
		})
		return
	}

	// Structural validation
	for _, verr := range pinfile.Validate(p) {
		result.Errors = append(result.Errors, &Error{
			Category:   CategoryPinfile,
			Path:       "craft.pin.yaml",
			Message:    verr.Error(),
			Suggestion: "Fix the pinfile field value",
		})
	}

	// Consistency check (FR-005): match manifest dep URLs against pinfile keys
	if len(m.Dependencies) == 0 && len(p.Resolved) > 0 {
		result.Warnings = append(result.Warnings, &Warning{
			Message: "craft.pin.yaml exists but craft.yaml has no dependencies — pinfile may be unnecessary",
		})
		return
	}

	// Each manifest dependency should have a pinfile entry
	depURLs := make(map[string]bool)
	for _, url := range m.Dependencies {
		depURLs[url] = true
		if _, ok := p.Resolved[url]; !ok {
			result.Errors = append(result.Errors, &Error{
				Category:   CategoryPinfile,
				Path:       "craft.pin.yaml",
				Field:      url,
				Message:    "manifest dependency has no matching pinfile entry",
				Suggestion: "Run 'craft install' to regenerate the pinfile",
			})
		}
	}

	// Each pinfile entry should have a manifest dependency
	// (transitive entries with a Source field are exempt)
	for url, entry := range p.Resolved {
		if entry.Source != "" {
			continue // transitive dependency — not expected in manifest
		}
		if !depURLs[url] {
			result.Errors = append(result.Errors, &Error{
				Category:   CategoryPinfile,
				Path:       "craft.pin.yaml",
				Field:      url,
				Message:    "pinfile entry has no matching manifest dependency",
				Suggestion: "Remove the stale entry or add the dependency to craft.yaml",
			})
		}
	}
}

// checkNameCollisions detects duplicate skill names across skill paths.
func (r *Runner) checkNameCollisions(result *Result, skillNames map[string][]string) {
	for name, paths := range skillNames {
		if len(paths) > 1 {
			result.Errors = append(result.Errors, &Error{
				Category: CategoryCollision,
				Field:    name,
				Message:  fmt.Sprintf("skill name %q exported by multiple paths: %s", name, strings.Join(paths, ", ")),
				Suggestion: "Each skill name must be unique within the package — rename one of the conflicting skills",
			})
		}
	}
}

package initcmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/erdemtuna/craft/internal/manifest"
)

// namePattern matches valid package names.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// semverPattern matches strict MAJOR.MINOR.PATCH version strings.
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Wizard runs the interactive craft init flow.
type Wizard struct {
	// Root is the directory where craft.yaml will be created.
	Root string

	// In is the input reader (usually os.Stdin).
	In io.Reader

	// Out is the output writer (usually os.Stdout).
	Out io.Writer

	// ErrOut is the error output writer (usually os.Stderr).
	ErrOut io.Writer
}

// NewWizard creates a Wizard for the given root directory.
func NewWizard(root string, in io.Reader, out, errOut io.Writer) *Wizard {
	return &Wizard{
		Root:   root,
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}
}

// Run executes the interactive init flow.
func (w *Wizard) Run() error {
	// Check if stdin is a terminal
	if f, ok := w.In.(*os.File); ok {
		if !isTerminal(f) {
			return fmt.Errorf("craft init requires an interactive terminal (TTY); cannot run in non-interactive mode")
		}
	}

	scanner := bufio.NewScanner(w.In)

	// Check for existing craft.yaml
	manifestPath := filepath.Join(w.Root, "craft.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		fmt.Fprintln(w.Out, "A craft.yaml already exists in this directory.")
		overwrite, err := w.promptYesNo(scanner, "Overwrite?", false)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(w.Out, "Aborted.")
			return nil
		}
	}

	// Infer defaults
	defaultName := inferPackageName(w.Root)

	fmt.Fprintln(w.Out, "Initializing a new craft package...")
	fmt.Fprintln(w.Out)

	// Prompt for name
	name, err := w.promptValidated(scanner, fmt.Sprintf("Package name (%s)", defaultName), defaultName, validateName)
	if err != nil {
		return err
	}

	// Prompt for version
	version, err := w.promptValidated(scanner, "Version (0.1.0)", "0.1.0", validateVersion)
	if err != nil {
		return err
	}

	// Prompt for description (no validation)
	description, err := w.prompt(scanner, "Description", "")
	if err != nil {
		return err
	}

	// Prompt for license (no validation)
	license, err := w.prompt(scanner, "License", "")
	if err != nil {
		return err
	}

	// Auto-discover skills
	fmt.Fprintln(w.Out)
	fmt.Fprintln(w.Out, "Discovering skill directories...")

	skills, err := DiscoverSkills(w.Root)
	if err != nil {
		fmt.Fprintf(w.ErrOut, "warning: error during skill discovery: %v\n", err)
		skills = nil
	}

	if len(skills) > 0 {
		fmt.Fprintf(w.Out, "Found %d skill(s):\n", len(skills))
		for _, s := range skills {
			fmt.Fprintf(w.Out, "  %s\n", s)
		}
	} else {
		fmt.Fprintln(w.Out, "No skill directories found.")
		skills = []string{}
	}

	// Build manifest
	m := &manifest.Manifest{
		SchemaVersion: 1,
		Name:          name,
		Version:       version,
		Description:   description,
		License:       license,
		Skills:        skills,
	}

	// Write manifest
	f, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("creating craft.yaml: %w", err)
	}
	defer f.Close()

	if err := manifest.Write(m, f); err != nil {
		return fmt.Errorf("writing craft.yaml: %w", err)
	}

	fmt.Fprintln(w.Out)
	fmt.Fprintf(w.Out, "Created craft.yaml for package %q (%s)\n", name, version)

	return nil
}

// prompt displays a prompt and returns the user's input or the default value.
func (w *Wizard) prompt(scanner *bufio.Scanner, label, defaultVal string) (string, error) {
	fmt.Fprintf(w.Out, "%s: ", label)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading input: %w", err)
		}
		return defaultVal, nil
	}

	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

// promptValidated displays a prompt and validates input, retrying on invalid input.
func (w *Wizard) promptValidated(scanner *bufio.Scanner, label, defaultVal string, validate func(string) error) (string, error) {
	for {
		val, err := w.prompt(scanner, label, defaultVal)
		if err != nil {
			return "", err
		}

		if verr := validate(val); verr != nil {
			fmt.Fprintf(w.ErrOut, "  invalid: %v\n", verr)
			continue
		}

		return val, nil
	}
}

// promptYesNo asks a yes/no question and returns the boolean answer.
func (w *Wizard) promptYesNo(scanner *bufio.Scanner, label string, defaultVal bool) (bool, error) {
	defaultStr := "y/N"
	if defaultVal {
		defaultStr = "Y/n"
	}

	fmt.Fprintf(w.Out, "%s [%s]: ", label, defaultStr)

	if !scanner.Scan() {
		return defaultVal, scanner.Err()
	}

	val := strings.TrimSpace(strings.ToLower(scanner.Text()))
	switch val {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		return defaultVal, nil
	default:
		return defaultVal, nil
	}
}

// inferPackageName derives a default package name from the directory path.
// Lowercases, replaces invalid characters with hyphens.
func inferPackageName(root string) string {
	name := strings.ToLower(filepath.Base(root))

	// Replace invalid characters with hyphens
	var result []rune
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}

	s := string(result)

	// Trim leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Ensure starts with a letter
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9') {
		s = "pkg-" + s
	}

	if s == "" {
		s = "my-package"
	}

	return s
}

// validateName checks if a string is a valid package name.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 128 {
		return fmt.Errorf("name must be 1–128 characters")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("must be lowercase alphanumeric with hyphens (e.g. 'my-package')")
	}
	return nil
}

// validateVersion checks if a string is a valid semver version.
func validateVersion(ver string) error {
	if ver == "" {
		return fmt.Errorf("version is required")
	}
	if !semverPattern.MatchString(ver) {
		return fmt.Errorf("must be MAJOR.MINOR.PATCH (e.g. '1.0.0')")
	}
	return nil
}

// isTerminal checks if the given file is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

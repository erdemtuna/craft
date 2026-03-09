package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/validate"
	"github.com/spf13/cobra"
)

// validateCmd runs all validation checks on the current craft package
// and reports errors to stderr with actionable suggestions.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a craft package",
	Long:  "Run all validation checks on the current craft package: schema, skill paths, frontmatter, dependency URLs, pinfile consistency, and name collisions.\nWith --global/-g: validate the global manifest and pinfile.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if globalFlag {
			return runValidateGlobal(cmd)
		}

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		runner := validate.NewRunner(root)
		result := runner.Run()

		// Print warnings
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
		}

		// Print errors
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "error: %s\n", e.Error())
			if e.Suggestion != "" {
				fmt.Fprintf(os.Stderr, "  hint: %s\n", e.Suggestion)
			}
		}

		if result.OK() {
			cmd.Println("✓ Package is valid")
			return nil
		}

		fmt.Fprintf(os.Stderr, "\n%d validation error(s) found\n", len(result.Errors))
		return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
	},
}

func runValidateGlobal(cmd *cobra.Command) error {
	manifestPath, err := GlobalManifestPath()
	if err != nil {
		return err
	}

	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no global manifest found\n  hint: use `craft get` to install skills")
		}
		return fmt.Errorf("reading global craft.yaml: %w", err)
	}

	errs := manifest.ValidateGlobal(m)
	if len(errs) == 0 {
		cmd.Println("✓ Global manifest is valid")

		// Also check pinfile if it exists
		pfPath, err := GlobalPinfilePath()
		if err != nil {
			return err
		}
		if _, err := pinfile.ParseFile(pfPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "warning: global pinfile issue: %v\n", err)
			}
		}
		return nil
	}

	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "error: %s\n", e.Error())
	}
	fmt.Fprintf(os.Stderr, "\n%d validation error(s) found\n", len(errs))
	return fmt.Errorf("validation failed with %d error(s)", len(errs))
}

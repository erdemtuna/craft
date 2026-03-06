package cli

import (
	"fmt"
	"os"

	"github.com/erdemtuna/craft/internal/validate"
	"github.com/spf13/cobra"
)

// validateCmd runs all validation checks on the current craft package
// and reports errors to stderr with actionable suggestions.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a craft package",
	Long:  "Run all validation checks on the current craft package: schema, skill paths, frontmatter, dependency URLs, pinfile consistency, and name collisions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
				fmt.Fprintf(os.Stderr, "  fix: %s\n", e.Suggestion)
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

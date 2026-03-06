package cli

import (
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a craft package",
	Long:  "Run all validation checks on the current craft package: schema, skill paths, frontmatter, dependency URLs, pinfile consistency, and name collisions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("craft validate is not yet implemented")
		return nil
	},
}

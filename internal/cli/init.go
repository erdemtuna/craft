package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new craft package",
	Long:  "Create a craft.yaml manifest file in the current directory through interactive prompts.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("craft init is not yet implemented")
		return nil
	},
}

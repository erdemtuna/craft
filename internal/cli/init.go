package cli

import (
	"os"

	initcmd "github.com/erdemtuna/craft/internal/init"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new craft package",
	Long:  "Create a craft.yaml manifest file in the current directory through interactive prompts.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := os.Getwd()
		if err != nil {
			return err
		}

		wizard := initcmd.NewWizard(root, os.Stdin, cmd.OutOrStdout(), os.Stderr)
		return wizard.Run()
	},
}

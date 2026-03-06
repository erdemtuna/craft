package cli

import (
	"github.com/erdemtuna/craft/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the craft version",
	Long:  "Display the current version of the craft CLI tool.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("craft version %s\n", version.Version)
	},
}

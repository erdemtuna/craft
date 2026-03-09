package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "craft",
	Short:         "Agent Skills package manager",
	Long:          "craft — resolve, install, and manage skill dependencies for Agent Skills packages.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose diagnostic output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(outdatedCmd)
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		// Don't print silent exit errors — they signal a non-zero
		// exit code without an error message (e.g., craft outdated).
		if _, ok := err.(*silentExitError); !ok {
			fmt.Fprintln(os.Stderr, err)
		}
		return err
	}
	return nil
}

// silentExitError signals a non-zero exit code without printing an error message.
// Used by commands like `craft outdated` that use exit code 1 as a signal (not an error).
// The code field is currently always 1; it exists for future extensibility.
type silentExitError struct {
	code int
}

func (e *silentExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

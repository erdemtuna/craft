package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// verbose controls whether verbose diagnostic output is enabled.
var verbose bool

// verboseLog writes a message to stderr when verbose mode is enabled.
func verboseLog(cmd *cobra.Command, format string, args ...any) {
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
	}
}

package cli

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// verbose controls whether verbose diagnostic output is enabled.
var verbose bool

// verboseLog writes a message to stderr when verbose mode is enabled.
func verboseLog(cmd *cobra.Command, format string, args ...any) {
	if verbose {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
	}
}

// sanitize strips control characters (except tab and newline) from a string
// to prevent terminal escape injection from untrusted input like dependency
// aliases, URLs, and skill names.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

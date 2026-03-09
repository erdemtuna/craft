package main

import (
	"os"

	"github.com/erdemtuna/craft/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}

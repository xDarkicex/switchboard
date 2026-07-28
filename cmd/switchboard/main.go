// Package main is the switchboard CLI entry point. The binary is the operator's
// cord board: every subcommand is a piece of equipment the operator uses on
// shift.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the top of the cobra command tree. The actual work lives in the
// subcommands wired up in init() blocks across this package.
var rootCmd = &cobra.Command{
	Use:   "switchboard",
	Short: "A general-purpose AI / API gateway and classifier-proxy.",
	Long: `switchboard listens on the line, classifies the call with a pre-trained
steady model, and patches the call through to the right upstream backend.

The architecture is vintage: every internal package is named after a piece of
the operator's equipment. The code is fast, the names are fun.`,
	Version:      "0.1.1",
	SilenceUsage: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

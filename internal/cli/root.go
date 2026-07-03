// Package cli implements the command-line interface for godisplacementx.
package cli

import (
	"github.com/spf13/cobra"
)

// Version is the application version, overridable at build time via -ldflags.
var Version = "0.1.0"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "godisplacementx",
		Short: "Procedural displacement / sci-fi height-map generator",
		Long: `godisplacementx generates procedural grayscale displacement / height maps
(a Go port of Displacement X / JSplacement) for use in 3D software such as
Blender, Cinema4D and Octane.

Run without a subcommand to open the GUI; use "generate" for headless output.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newBundleCmd())
	root.AddCommand(newRandomizeCmd())
	root.AddCommand(newVersionCmd())
	return root
}

// Execute runs the CLI.
func Execute() error {
	return newRootCmd().Execute()
}

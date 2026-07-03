// Command godisplacementx is a procedural displacement / sci-fi height-map
// generator: a Go port of Displacement X (a web alternative to JSplacement).
//
// With no arguments it opens the GUI; with a subcommand it runs headless on the
// command line (see `godisplacementx generate --help`).
package main

import (
	"fmt"
	"os"

	"godisplacementx/internal/cli"
)

func main() {
	// Any arguments => run the CLI. No arguments => open the GUI.
	if len(os.Args) > 1 {
		attachParentConsole()
		if err := cli.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if err := runGUI(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

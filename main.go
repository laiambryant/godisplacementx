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

var exitFunc = os.Exit

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		exitFunc(1)
	}
}

// run dispatches on the invocation shape: any arguments => the CLI, no
// arguments => the GUI.
func run() error {
	if len(os.Args) > 1 {
		attachParentConsole()
		return cli.Execute()
	}
	return runGUI()
}

//go:build !desktop && !bindings

package main

import "errors"

// runGUI is a stub for builds without the GUI (plain `go build`). The desktop
// build (via `wails build` / `wails dev`, which set the `desktop` tag) provides
// the real implementation in gui.go.
func runGUI() error {
	return errors.New("this build has no GUI; build the desktop app with `wails build` (or run `wails dev`)")
}

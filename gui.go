//go:build desktop || bindings

package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/public
var assets embed.FS

// runGUI launches the Wails desktop application. The UI is plain HTML + htmx:
// Wails serves the embedded static files, and anything it can't find (every
// /api/* route, all POSTs) falls through to the Go handler in gui_handler.go.
func runGUI() error {
	app := NewApp()
	return wails.Run(&options.App{
		Title:            "godisplacementx",
		Width:            1280,
		Height:           900,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets, Handler: app.routes()},
		BackgroundColour: &options.RGBA{R: 0x0f, G: 0x13, B: 0x17, A: 255},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
}

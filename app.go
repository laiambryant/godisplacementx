package main

import (
	"context"
	"sync"
)

// App is the Wails-bound application object. It holds the runtime context (so the
// GUI handler can open native dialogs) and the most recent render's PNG bytes,
// which are served by /api/image and reused by Save. All UI logic lives in
// gui_handler.go.
type App struct {
	ctx context.Context

	mu        sync.Mutex
	lastPNG   []byte
	renderVer int
}

// NewApp creates the application object.
func NewApp() *App { return &App{} }

// startup stores the Wails runtime context.
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

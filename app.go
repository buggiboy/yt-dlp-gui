package main

import (
	"context"
)

// App struct
type App struct {
	ctx         context.Context
	previewPort int
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Best-effort: if this fails, previewPort stays 0 and the UI simply
	// doesn't show the preview. Downloads are unaffected.
	_ = a.startPreviewServer()
}

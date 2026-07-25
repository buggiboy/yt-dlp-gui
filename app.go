package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	previewPort int

	// dlMu guards the running-download fields below. DownloadVideo runs on one
	// goroutine while CancelDownload (and shutdown) arrive on another, so the
	// handle to the child process has to be shared safely.
	dlMu sync.Mutex
	// dlCmd is the yt-dlp process currently downloading, or nil when idle.
	dlCmd *exec.Cmd
	// dlCancelled records that the current process was killed on purpose, so
	// DownloadVideo can report a cancellation instead of a crash.
	dlCancelled bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Carry a pre-rename install over to the current folder name. Must run
	// before anything reads appDir(), so it goes first.
	_ = migrateLegacyAppDir()
	// Best-effort: if this fails, previewPort stays 0 and the UI simply
	// doesn't show the preview. Downloads are unaffected.
	_ = a.startPreviewServer()
}

// OpenExternalURL opens a link in the user's default browser. Bound to the
// frontend so links (currently just yt-dlp's changelog) leave the app window
// instead of navigating the webview away from the UI.
//
// The allowlist is the point: the frontend should only ever pass URLs the
// backend itself produced, and pinning the prefix keeps a bug elsewhere from
// turning this into a "launch anything" hole.
func (a *App) OpenExternalURL(url string) error {
	if !strings.HasPrefix(url, ytDlpRepoURL+"/") {
		return fmt.Errorf("refusing to open %s — only yt-dlp project links are allowed", url)
	}
	wailsruntime.BrowserOpenURL(a.ctx, url)
	return nil
}

// shutdown is called as the app closes. A download in flight would otherwise
// outlive the window as an orphaned process (still writing to disk, still using
// bandwidth, with no UI left to stop it), so we kill it on the way out.
func (a *App) shutdown(ctx context.Context) {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	if a.dlCmd != nil {
		a.dlCancelled = true
		killProcessTree(a.dlCmd)
	}
}

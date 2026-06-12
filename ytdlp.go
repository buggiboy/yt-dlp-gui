package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	// The Wails runtime package is also named "runtime", so we alias it to
	// avoid clashing with Go's standard library "runtime" used above.
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// progressSentinel marks lines produced by our custom yt-dlp progress template
// so we can tell them apart from yt-dlp's normal log output.
const progressSentinel = "__YTPGUI_PROGRESS__"

// ansiRe strips terminal color codes that yt-dlp sometimes embeds in its
// progress strings, so percentages/speeds parse cleanly.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// DownloadProgress is the payload we emit to the frontend on each update.
type DownloadProgress struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	Eta     string  `json:"eta"`
}

// ytDlpAssetName returns the correct yt-dlp release asset filename for the
// current operating system and CPU architecture. yt-dlp publishes a separate
// standalone binary per platform on its GitHub releases page.
func ytDlpAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return "yt-dlp.exe"
	case "darwin":
		// The _macos build is a universal binary (works on Intel + Apple Silicon).
		return "yt-dlp_macos"
	default: // linux and others
		if runtime.GOARCH == "arm64" {
			return "yt-dlp_linux_aarch64"
		}
		return "yt-dlp_linux"
	}
}

// ytDlpPath returns the full path where we store the managed yt-dlp binary.
// We keep it in the OS user-config dir (e.g. ~/Library/Application Support on
// macOS) under our app's folder, so each user has their own copy.
func ytDlpPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name = "yt-dlp.exe"
	}
	return filepath.Join(cfgDir, "ytpgui", name), nil
}

// downloadsDir returns the current user's Downloads folder.
func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// YtDlpInstalled reports whether the managed yt-dlp binary is present on disk.
// Bound to the frontend so the UI can show install state.
func (a *App) YtDlpInstalled() bool {
	p, err := ytDlpPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// InstallYtDlp downloads the latest yt-dlp binary for this platform from the
// official GitHub releases and makes it executable. Returns the version string
// on success. Bound to the frontend "Download latest yt-dlp" button.
func (a *App) InstallYtDlp() (string, error) {
	p, err := ytDlpPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", fmt.Errorf("could not create app folder: %w", err)
	}

	url := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + ytDlpAssetName()
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: server returned %s", resp.Status)
	}

	// Write to a temp file first, then atomically rename into place so a failed
	// download never leaves a half-written binary.
	tmp := p + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", err
	}
	out.Close()

	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return "", err
	}

	return a.YtDlpVersion()
}

// YtDlpVersion runs `yt-dlp --version` and returns the trimmed output.
func (a *App) YtDlpVersion() (string, error) {
	p, err := ytDlpPath()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(p, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("could not run yt-dlp: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DownloadVideo runs yt-dlp on the given URL, saving the result into the user's
// Downloads folder. It streams live progress to the frontend via Wails events
// ("download:progress" and "download:log") while the process runs, and returns
// once the download finishes. Bound to the frontend "Download" button.
func (a *App) DownloadVideo(url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("please enter a URL")
	}
	if !a.YtDlpInstalled() {
		return "", fmt.Errorf("yt-dlp is not installed yet — click \"Download latest yt-dlp\" first")
	}

	bin, err := ytDlpPath()
	if err != nil {
		return "", err
	}
	dir, err := downloadsDir()
	if err != nil {
		return "", err
	}

	// Output template: save as "<video title>.<extension>" in Downloads.
	outTmpl := filepath.Join(dir, "%(title)s.%(ext)s")

	// --newline makes yt-dlp print each progress update on its own line (instead
	// of redrawing one line with carriage returns), so we can read them cleanly.
	// --progress-template emits our own machine-readable line we can parse.
	progressTmpl := "download:" + progressSentinel +
		"|%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s"

	cmd := exec.Command(
		bin,
		"--newline",
		"--progress-template", progressTmpl,
		"-o", outTmpl,
		url,
	)

	// Funnel both stdout and stderr into one pipe so we read everything in order.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("could not start yt-dlp: %w", err)
	}

	// Wait for the process in a goroutine and close the pipe writer when it
	// exits, which lets the scanner below reach EOF and stop.
	done := make(chan error, 1)
	go func() {
		werr := cmd.Wait()
		pw.Close()
		done <- werr
	}()

	var lastErrLine string
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := ansiRe.ReplaceAllString(scanner.Text(), "")
		if strings.HasPrefix(line, progressSentinel+"|") {
			a.emitProgress(line)
			continue
		}
		// Any other line is a human-readable status (e.g. "[download] Destination:",
		// "[Merger] Merging formats"). Surface it so the UI can show the step.
		wailsruntime.EventsEmit(a.ctx, "download:log", line)
		if strings.HasPrefix(strings.ToUpper(line), "ERROR") {
			lastErrLine = line
		}
	}

	if werr := <-done; werr != nil {
		if lastErrLine != "" {
			return "", fmt.Errorf("%s", lastErrLine)
		}
		return "", fmt.Errorf("yt-dlp exited with an error: %w", werr)
	}

	// Make sure the bar finishes at 100%.
	wailsruntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{Percent: 100, Eta: "0:00"})
	return "Done", nil
}

// emitProgress parses one of our sentinel progress lines and forwards it to the
// frontend. Line format: "__YTPGUI_PROGRESS__| 45.2%|1.20MiB/s|00:12".
func (a *App) emitProgress(line string) {
	parts := strings.Split(line, "|")
	if len(parts) < 4 {
		return
	}
	pctStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), "%")
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return // e.g. "N/A" before totals are known — just skip this tick.
	}
	wailsruntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Percent: pct,
		Speed:   strings.TrimSpace(parts[2]),
		Eta:     strings.TrimSpace(parts[3]),
	})
}

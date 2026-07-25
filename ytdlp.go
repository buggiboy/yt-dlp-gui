package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// progressSentinel marks lines produced by our custom yt-dlp progress template
// so we can tell them apart from yt-dlp's normal log output.
const progressSentinel = "__YTDLP_GUI_PROGRESS__"

// ansiRe strips terminal color codes that yt-dlp sometimes embeds in its
// progress strings, so percentages/speeds parse cleanly.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// DownloadProgress is the payload we emit to the frontend on each update.
type DownloadProgress struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	Eta     string  `json:"eta"`
}

// timestampRe accepts plain seconds ("90", "90.5") or clock-style stamps
// ("1:30", "01:02:30", "1:02:30.5").
var timestampRe = regexp.MustCompile(`^(\d+(\.\d+)?|(\d+:)?[0-5]?\d:[0-5]\d(\.\d+)?)$`)

// parseTimestamp converts a user-entered timestamp to seconds.
// Accepts "SS", "MM:SS", or "HH:MM:SS" (fractions allowed on the last part).
func parseTimestamp(ts string) (float64, error) {
	ts = strings.TrimSpace(ts)
	if !timestampRe.MatchString(ts) {
		return 0, fmt.Errorf("invalid timestamp %q — use seconds (90), MM:SS (1:30), or HH:MM:SS (1:02:30)", ts)
	}
	parts := strings.Split(ts, ":")
	var total float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp %q", ts)
		}
		total = total*60 + v
	}
	return total, nil
}

// downloadsDir returns the current user's Downloads folder.
func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// DownloadsFolder exposes the default save location to the frontend so it can
// show where files will land before the user picks a custom folder.
func (a *App) DownloadsFolder() (string, error) {
	return downloadsDir()
}

// ChooseDownloadFolder opens the OS-native folder picker and returns the
// chosen path, or "" if the user cancelled. Bound to the frontend "Choose…"
// button next to the save location.
func (a *App) ChooseDownloadFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose download folder",
	})
}

// YtDlpInstalled reports whether the app's Python runtime and the yt-dlp zipapp
// are both present. Bound to the frontend so the UI can show install state.
func (a *App) YtDlpInstalled() bool {
	return runtimeReady()
}

// InstallYtDlp downloads whatever's missing — the standalone Python interpreter
// and/or the yt-dlp zipapp — into the app's folder, reporting progress to the
// frontend, then returns the yt-dlp version. Bound to the frontend
// "Download latest yt-dlp" button.
func (a *App) InstallYtDlp() (string, error) {
	if err := a.ensureRuntime(); err != nil {
		return "", err
	}
	return a.YtDlpVersion()
}

// YtDlpVersion runs yt-dlp's --version through our bundled interpreter.
func (a *App) YtDlpVersion() (string, error) {
	cmd, err := ytDlpCmd("--version")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not run yt-dlp: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// VideoFormat describes one selectable resolution: the height plus the
// container extension (mp4, webm, …) of the best format at that height.
type VideoFormat struct {
	Height int    `json:"height"`
	Ext    string `json:"ext"`
}

// FormatList is ListFormats' response: distinct video heights sorted high→low
// with their container, plus the container of the best audio-only format.
type FormatList struct {
	Videos   []VideoFormat `json:"videos"`
	AudioExt string        `json:"audioExt"`
}

// ListFormats asks yt-dlp (metadata only, no download) which formats are
// actually available for a URL. The frontend uses this to narrow its preset
// quality dropdown to what the video really offers. Bound to the frontend.
//
// CheckURL returns the same list from its own probe, so the UI normally gets
// formats for free while validating; this remains the standalone entry point.
func (a *App) ListFormats(rawURL string) (FormatList, error) {
	var list FormatList
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return list, fmt.Errorf("empty URL")
	}
	if !runtimeReady() {
		return list, fmt.Errorf("yt-dlp is not installed yet")
	}

	out, err := a.probeURL(rawURL)
	if err != nil {
		return list, fmt.Errorf("could not read formats: %w", err)
	}
	list, err = decodeFormatList(out)
	if err != nil {
		return FormatList{}, err
	}
	return list, nil
}

// decodeFormatList parses a yt-dlp --dump-json blob into the selectable video
// heights and the best audio-only container. Split out so ListFormats and
// CheckURL agree on exactly what a format list means. (The same blob's
// descriptive metadata is read separately by decodePreviewMeta.)
func decodeFormatList(out []byte) (FormatList, error) {
	var list FormatList
	var meta struct {
		Formats []struct {
			Vcodec string `json:"vcodec"`
			Acodec string `json:"acodec"`
			Height int    `json:"height"`
			Ext    string `json:"ext"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(out, &meta); err != nil {
		return list, fmt.Errorf("could not parse format list: %w", err)
	}

	// yt-dlp lists formats worst→best, so for each height the LAST format we
	// see is the best one — letting later entries overwrite earlier ones means
	// we end up with the ext of the best format at each height.
	extByHeight := map[int]string{}
	for _, f := range meta.Formats {
		if f.Vcodec != "none" && f.Height > 0 {
			extByHeight[f.Height] = f.Ext
		} else if f.Vcodec == "none" && f.Acodec != "none" && f.Ext != "" {
			list.AudioExt = f.Ext // audio-only: keep the last (= best) ext
		}
	}

	heights := make([]int, 0, len(extByHeight))
	for h := range extByHeight {
		heights = append(heights, h)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(heights)))
	list.Videos = make([]VideoFormat, len(heights))
	for i, h := range heights {
		list.Videos[i] = VideoFormat{Height: h, Ext: extByHeight[h]}
	}
	return list, nil
}

// audioFormats is the allowlist of conversion targets we expose in the UI.
// Anything here is handed to ffmpeg via yt-dlp's --audio-format.
var audioFormats = map[string]bool{
	"mp3":  true,
	"m4a":  true,
	"opus": true,
	"flac": true,
}

// qualityArgs translates the frontend's quality choice into yt-dlp arguments.
//   - "" / "best": yt-dlp's default (best video + best audio).
//   - "audio": best audio only; extracted to an audio file when ffmpeg exists.
//     audioFormat ("" = keep native codec, or one of audioFormats) selects a
//     conversion target; it's ignored for non-audio downloads and without
//     ffmpeg (the UI disables the picker in that case, so no error needed).
//   - "<height>" (e.g. "1080"): prefer the largest format at or below that
//     height. -S "res:H" sorts rather than filters, so if nothing is at or
//     below H, yt-dlp gracefully takes the smallest format above it instead
//     of failing.
//
// ffmpegAvailable is passed in rather than looked up so this stays a pure
// function — see args.go.
func qualityArgs(quality, audioFormat string, ffmpegAvailable bool) ([]string, error) {
	switch q := strings.TrimSpace(quality); q {
	case "", "best":
		return nil, nil
	case "audio":
		args := []string{"-f", "bestaudio/best"}
		if ffmpegAvailable {
			// -x strips the video container and keeps the native audio codec
			// (m4a/opus); without ffmpeg we just download the audio stream as-is.
			args = append(args, "-x")
			if f := strings.TrimSpace(audioFormat); f != "" {
				if !audioFormats[f] {
					return nil, fmt.Errorf("invalid audio format %q", audioFormat)
				}
				// --audio-quality 0 = best VBR setting for lossy targets
				// (yt-dlp's default is 5, a middling bitrate); harmless no-op
				// for flac.
				args = append(args, "--audio-format", f, "--audio-quality", "0")
			}
		}
		return args, nil
	default:
		h, err := strconv.Atoi(q)
		if err != nil || h <= 0 {
			return nil, fmt.Errorf("invalid quality %q", quality)
		}
		return []string{"-S", fmt.Sprintf("res:%d", h)}, nil
	}
}

// DownloadVideo runs yt-dlp on the given URL, saving the result into
// opts.Folder (the user's Downloads folder by default). If Start and/or End
// are non-empty, only that section of the video is downloaded (requires
// ffmpeg). It streams live progress to the frontend via Wails events
// ("download:progress" and "download:log") while the process runs, and
// returns once the download finishes.
//
// The arguments themselves are assembled by buildArgs (args.go) — the same
// function PreviewCommand uses, so what the user was shown is what runs. All
// this adds is the plumbing that makes the output machine-readable. Bound to
// the frontend "Download" button.
func (a *App) DownloadVideo(opts DownloadOptions) (string, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return "", fmt.Errorf("please enter a URL")
	}
	if !a.YtDlpInstalled() {
		return "", fmt.Errorf("yt-dlp is not installed yet — click \"Download latest yt-dlp\" first")
	}

	dir, err := resolveFolder(opts.Folder)
	if err != nil {
		return "", err
	}
	// The folder was picked via the OS dialog, so it existed then — but it may
	// have been deleted or unmounted since.
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("the chosen folder no longer exists: %s", dir)
	}

	userArgs, err := buildArgs(opts, argEnv{
		ffmpegAvailable: a.FfmpegAvailable(),
		dir:             dir,
		// A real download refuses options it can't make sense of rather than
		// quietly dropping them — the opposite of the preview.
		lenient: false,
	})
	if err != nil {
		return "", err
	}

	// --newline makes yt-dlp print each progress update on its own line (instead
	// of redrawing one line with carriage returns), so we can read them cleanly.
	// --progress-template emits our own machine-readable line we can parse.
	// Neither is a user choice, so neither appears in the command preview.
	progressTmpl := "download:" + progressSentinel +
		"|%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s"
	args := []string{"--newline", "--progress-template", progressTmpl}
	// Point yt-dlp at our managed ffmpeg if we downloaded one (no-op when the
	// user skipped it or relies on a system install). Placed here, ahead of the
	// user's own arguments, so a --ffmpeg-location in Extra arguments still wins.
	args = append(args, ffmpegLocationArgs()...)
	args = append(args, userArgs...)

	cmd, err := ytDlpCmd(args...)
	if err != nil {
		return "", err
	}

	// Funnel both stdout and stderr into one pipe so we read everything in order.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("could not start yt-dlp: %w", err)
	}

	// Publish the handle so CancelDownload (and app shutdown) can reach the
	// process, and clear it again however this function returns.
	a.setActiveDownload(cmd)
	defer a.clearActiveDownload()

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
	// yt-dlp occasionally prints a very long line (a format table, a dumped
	// URL, a stack trace). The default 64 KiB limit would abort the scan, and
	// with nothing left draining the pipe the process would block on write and
	// cmd.Wait would never return — hanging the download with no way out.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
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
	scanErr := scanner.Err()
	// Close the read half before waiting. On a normal finish this is a no-op
	// (the writer already closed at EOF); after a scan error it unblocks the
	// process so cmd.Wait can return instead of deadlocking.
	pr.Close()

	werr := <-done
	if a.wasCancelled() {
		return "", fmt.Errorf("download cancelled")
	}
	if werr != nil {
		if lastErrLine != "" {
			return "", fmt.Errorf("%s", lastErrLine)
		}
		return "", fmt.Errorf("yt-dlp exited with an error: %w", werr)
	}
	if scanErr != nil {
		return "", fmt.Errorf("lost track of yt-dlp's output: %w", scanErr)
	}

	// Make sure the bar finishes at 100%.
	wailsruntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{Percent: 100, Eta: "0:00"})
	return "Done", nil
}

// --- cancellation -------------------------------------------------------------

// setActiveDownload records the running process so it can be cancelled, and
// resets the cancelled flag for this run.
func (a *App) setActiveDownload(cmd *exec.Cmd) {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	a.dlCmd = cmd
	a.dlCancelled = false
}

// clearActiveDownload forgets the process once it has exited. The cancelled
// flag is deliberately left set; setActiveDownload resets it when the next
// download starts.
func (a *App) clearActiveDownload() {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	a.dlCmd = nil
}

// wasCancelled reports whether the download that just ended was killed on
// purpose rather than failing on its own.
func (a *App) wasCancelled() bool {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	return a.dlCancelled
}

// CancelDownload stops the download in progress, killing yt-dlp and any ffmpeg
// it spawned. Partial files are left where yt-dlp put them (its .part files are
// resumable, so a re-run picks up rather than starting over). Doing nothing when
// no download is running is not an error — the button may simply have been
// clicked as the download finished. Bound to the frontend "Cancel" button.
func (a *App) CancelDownload() error {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	if a.dlCmd == nil {
		return nil
	}
	a.dlCancelled = true
	killProcessTree(a.dlCmd)
	return nil
}

// emitProgress parses one of our sentinel progress lines and forwards it to the
// frontend. Line format: "__YTDLP_GUI_PROGRESS__| 45.2%|1.20MiB/s|00:12".
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

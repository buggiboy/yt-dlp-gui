package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
func (a *App) ListFormats(rawURL string) (FormatList, error) {
	var list FormatList
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return list, fmt.Errorf("empty URL")
	}
	if !runtimeReady() {
		return list, fmt.Errorf("yt-dlp is not installed yet")
	}

	cmd, err := ytDlpCmd("--dump-json", "--no-playlist", "--playlist-items", "1", "--no-warnings", rawURL)
	if err != nil {
		return list, err
	}
	// Metadata extraction should be quick; kill it if a site makes it crawl.
	timer := time.AfterFunc(30*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill() //nolint:errcheck
		}
	})
	defer timer.Stop()

	out, err := cmd.Output()
	if err != nil {
		return list, fmt.Errorf("could not read formats: %w", err)
	}

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

// qualityArgs translates the frontend's quality choice into yt-dlp arguments.
//   - "" / "best": yt-dlp's default (best video + best audio).
//   - "audio": best audio only; extracted to an audio file when ffmpeg exists.
//   - "<height>" (e.g. "1080"): prefer the largest format at or below that
//     height. -S "res:H" sorts rather than filters, so if nothing is at or
//     below H, yt-dlp gracefully takes the smallest format above it instead
//     of failing.
func (a *App) qualityArgs(quality string) ([]string, error) {
	switch q := strings.TrimSpace(quality); q {
	case "", "best":
		return nil, nil
	case "audio":
		args := []string{"-f", "bestaudio/best"}
		if a.FfmpegAvailable() {
			// -x strips the video container and keeps the native audio codec
			// (m4a/opus); without ffmpeg we just download the audio stream as-is.
			args = append(args, "-x")
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

// DownloadVideo runs yt-dlp on the given URL, saving the result into the user's
// Downloads folder. If start and/or end are non-empty, only that section of the
// video is downloaded (requires ffmpeg). It streams live progress to the
// frontend via Wails events ("download:progress" and "download:log") while the
// process runs, and returns once the download finishes. quality is "best",
// "audio", or a max video height like "1080" (see qualityArgs). Bound to the
// frontend "Download" button.
func (a *App) DownloadVideo(url, start, end, quality string) (string, error) {
	url = strings.TrimSpace(url)
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if url == "" {
		return "", fmt.Errorf("please enter a URL")
	}
	if !a.YtDlpInstalled() {
		return "", fmt.Errorf("yt-dlp is not installed yet — click \"Download latest yt-dlp\" first")
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

	args := []string{
		"--newline",
		"--progress-template", progressTmpl,
		"-o", outTmpl,
	}

	qArgs, err := a.qualityArgs(quality)
	if err != nil {
		return "", err
	}
	args = append(args, qArgs...)

	// Point yt-dlp at our managed ffmpeg if we downloaded one (no-op when the
	// user skipped it or relies on a system install).
	args = append(args, ffmpegLocationArgs()...)

	// Section download: build a "*START-END" spec for --download-sections.
	if start != "" || end != "" {
		if !a.FfmpegAvailable() {
			return "", fmt.Errorf("downloading a section requires ffmpeg, which wasn't found on your system — install it (e.g. from ffmpeg.org) or clear the start/stop fields")
		}

		startSec, endSec := 0.0, -1.0
		from, to := "0", "inf"
		if start != "" {
			if startSec, err = parseTimestamp(start); err != nil {
				return "", fmt.Errorf("start time: %w", err)
			}
			from = start
		}
		if end != "" {
			if endSec, err = parseTimestamp(end); err != nil {
				return "", fmt.Errorf("stop time: %w", err)
			}
			to = end
		}
		if endSec >= 0 && endSec <= startSec {
			return "", fmt.Errorf("stop time must be after start time")
		}

		args = append(args,
			"--download-sections", fmt.Sprintf("*%s-%s", from, to),
			// Re-encode just around the cut points so the clip starts and ends
			// exactly where requested instead of snapping to the nearest keyframe.
			"--force-keyframes-at-cuts",
		)
	}

	args = append(args, url)
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

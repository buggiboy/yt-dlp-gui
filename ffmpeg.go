package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// This file manages an optional app-owned ffmpeg, mirroring how pyruntime.go
// manages Python: downloaded once into the app's folder, never touching the
// system. ffmpeg is only required for section downloads (--download-sections)
// and audio extraction (-x), so the UI lets the user skip it.
//
// Sources:
//   - macOS / Linux: ffmpeg.martin-riedl.de — static, signed (macOS) builds
//     with stable "redirect/latest" URLs meant for scripting.
//   - Windows: github.com/yt-dlp/FFmpeg-Builds — the ffmpeg builds the yt-dlp
//     project itself maintains and recommends.

// ffmpegBuildsWinURL is the yt-dlp project's recommended Windows build.
const ffmpegBuildsWinURL = "https://github.com/yt-dlp/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-win64-gpl.zip"

// ffmpegDir is where the managed binaries live: <appDir>/ffmpeg/.
func ffmpegDir() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ffmpeg"), nil
}

func ffmpegExePath() (string, error) {
	dir, err := ffmpegDir()
	if err != nil {
		return "", err
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name), nil
}

func ffprobeExePath() (string, error) {
	dir, err := ffmpegDir()
	if err != nil {
		return "", err
	}
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name), nil
}

// ffmpegSkipMarkerPath marks that the user chose "skip" for the optional
// ffmpeg download, so the setup checklist stops offering it on every launch.
func ffmpegSkipMarkerPath() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ffmpeg-skipped"), nil
}

// managedFfmpeg reports whether our own downloaded ffmpeg exists.
func managedFfmpeg() bool {
	exe, err := ffmpegExePath()
	return err == nil && fileExists(exe)
}

// FfmpegAvailable reports whether any usable ffmpeg exists — ours or one
// already on the system PATH. Bound to the frontend.
func (a *App) FfmpegAvailable() bool {
	if managedFfmpeg() {
		return true
	}
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// ffmpegLocationArgs returns the --ffmpeg-location flag pointing at our
// managed install, or nothing if we should rely on PATH.
func ffmpegLocationArgs() []string {
	if !managedFfmpeg() {
		return nil
	}
	dir, err := ffmpegDir()
	if err != nil {
		return nil
	}
	return []string{"--ffmpeg-location", dir}
}

// SkipFfmpeg records that the user declined the optional ffmpeg download.
// Bound to the frontend's "Skip" button.
func (a *App) SkipFfmpeg() error {
	marker, err := ffmpegSkipMarkerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("ok\n"), 0o644)
}

// InstallFfmpeg downloads ffmpeg (and ffprobe) into the app's folder. Bound
// to the frontend's per-dependency download button.
func (a *App) InstallFfmpeg() error {
	dir, err := ffmpegDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		if err := a.installFfmpegUnix(); err != nil {
			return err
		}
	case "windows":
		if err := a.installFfmpegWindows(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("no managed ffmpeg build for %s — install ffmpeg from your package manager", runtime.GOOS)
	}

	// A successful install supersedes any earlier "skip" choice.
	if marker, err := ffmpegSkipMarkerPath(); err == nil {
		_ = os.Remove(marker)
	}
	a.emitInstallProgress(100)
	return nil
}

// installFfmpegUnix fetches the ffmpeg and ffprobe zips (each contains a
// single static binary) from martin-riedl.de's stable redirect URLs.
func (a *App) installFfmpegUnix() error {
	osName := "macos"
	if runtime.GOOS == "linux" {
		osName = "linux"
	}
	// The build server offers exactly amd64 and arm64 for both OSes; refuse
	// anything else rather than downloading a binary that can't run.
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("no managed ffmpeg build for %s/%s — install ffmpeg from your package manager", runtime.GOOS, arch)
	}
	base := fmt.Sprintf("https://ffmpeg.martin-riedl.de/redirect/latest/%s/%s/release", osName, arch)

	ffmpegDest, err := ffmpegExePath()
	if err != nil {
		return err
	}
	ffprobeDest, err := ffprobeExePath()
	if err != nil {
		return err
	}

	// ffmpeg is the big one; treat it as the first 80% of the progress bar.
	a.emitLog("Downloading ffmpeg…")
	if err := a.downloadBinaryZip(base+"/ffmpeg.zip", "ffmpeg", ffmpegDest, 0, 80); err != nil {
		return fmt.Errorf("ffmpeg download failed: %w", err)
	}
	a.emitLog("Downloading ffprobe…")
	if err := a.downloadBinaryZip(base+"/ffprobe.zip", "ffprobe", ffprobeDest, 80, 100); err != nil {
		return fmt.Errorf("ffprobe download failed: %w", err)
	}
	return nil
}

// installFfmpegWindows fetches yt-dlp's recommended Windows build — one zip
// containing both ffmpeg.exe and ffprobe.exe under a bin/ directory. The
// win64 build also covers Windows-on-ARM, which runs x64 binaries under
// emulation; only 32-bit Windows is genuinely unsupported.
func (a *App) installFfmpegWindows() error {
	if runtime.GOARCH == "386" {
		return fmt.Errorf("no managed ffmpeg build for 32-bit Windows — install ffmpeg manually from ffmpeg.org")
	}
	a.emitLog("Downloading ffmpeg…")
	tmp, err := a.downloadToTemp(ffmpegBuildsWinURL, 0, 90)
	if err != nil {
		return fmt.Errorf("ffmpeg download failed: %w", err)
	}
	defer os.Remove(tmp)

	a.emitLog("Installing ffmpeg…")
	ffmpegDest, err := ffmpegExePath()
	if err != nil {
		return err
	}
	ffprobeDest, err := ffprobeExePath()
	if err != nil {
		return err
	}
	if err := extractFromZip(tmp, "ffmpeg.exe", ffmpegDest); err != nil {
		return err
	}
	return extractFromZip(tmp, "ffprobe.exe", ffprobeDest)
}

// downloadBinaryZip downloads a zip to a temp file and extracts the named
// binary out of it, reporting progress across [pctFrom, pctTo].
func (a *App) downloadBinaryZip(url, binName, dest string, pctFrom, pctTo float64) error {
	tmp, err := a.downloadToTemp(url, pctFrom, pctTo)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return extractFromZip(tmp, binName, dest)
}

// downloadToTemp streams a URL to a temp file, mapping download progress onto
// the [pctFrom, pctTo] slice of the progress bar.
func (a *App) downloadToTemp(url string, pctFrom, pctTo float64) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %s", resp.Status)
	}

	f, err := os.CreateTemp("", "ytpgui-dl-*.zip")
	if err != nil {
		return "", err
	}
	counted := &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		onTick: func(pct float64) {
			a.emitInstallProgress(pctFrom + pct/100*(pctTo-pctFrom))
		},
	}
	if _, err := io.Copy(f, counted); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// extractFromZip pulls the first entry whose base name matches binName out of
// a zip archive (at any depth — Windows builds nest under bin/) and writes it
// to dest with the executable bit set.
func extractFromZip(zipPath, binName, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != binName {
			continue
		}
		// Skip macOS resource-fork junk entries.
		if strings.Contains(entry.Name, "__MACOSX") {
			continue
		}
		src, err := entry.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		tmp := dest + ".tmp"
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			_ = os.Remove(tmp)
			return err
		}
		out.Close()
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("%s not found inside downloaded archive", binName)
}

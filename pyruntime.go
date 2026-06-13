package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// This file manages the app's private Python runtime. Instead of shipping the
// slow PyInstaller yt-dlp binary (which unpacks a whole interpreter on every
// run), we download a relocatable standalone CPython once into the user's app
// folder, plus the pure-Python yt-dlp "zipapp", and run "python yt-dlp ...".
// After the one-time download there's no per-run unpacking, so startup is fast.
//
// Everything lives in a writable folder outside the .app bundle, so none of it
// needs to be code-signed or notarized as part of our app.

// preferredPyMinor is the CPython 3.x minor version we prefer when a release
// offers several. yt-dlp needs 3.9+; 3.12 is a stable, well-supported choice.
const preferredPyMinor = 12

// pbsLatestURL points at python-build-standalone's machine-readable pointer to
// its newest release. We read the tag from here, then ask the GitHub API which
// assets that release contains so we never hardcode a URL that might 404.
const pbsLatestURL = "https://raw.githubusercontent.com/astral-sh/python-build-standalone/latest-release/latest-release.json"

// ytDlpZipURL is the pure-Python yt-dlp build (a zipapp, not a compiled binary).
// It's tiny and needs an interpreter to run — which is exactly what we provide.
const ytDlpZipURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"

// appDir is the per-user folder holding everything we manage (interpreter +
// yt-dlp). e.g. ~/Library/Application Support/ytpgui on macOS.
func appDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "ytpgui"), nil
}

// pythonRoot is where the extracted interpreter tree lives. The tarball's top
// level is a "python/" directory, so the interpreter ends up under runtime/python.
func pythonRoot() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "runtime"), nil
}

// pythonExePath is the interpreter we invoke.
func pythonExePath() (string, error) {
	root, err := pythonRoot()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "python", "python.exe"), nil
	}
	return filepath.Join(root, "python", "bin", "python3"), nil
}

// ytDlpZipPath is where we store the yt-dlp zipapp.
func ytDlpZipPath() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "yt-dlp"), nil
}

// extrasMarkerPath marks that yt-dlp's optional dependencies have been
// installed into the runtime. The zipapp ships with none of them, but several
// are effectively required for broad site support — most importantly
// curl_cffi, which provides the browser impersonation that sites like Vimeo
// demand before they'll serve video at all. Bump the "v1" if the extras list
// changes, and existing installs will re-run the step.
func extrasMarkerPath() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "extras-v1-ok"), nil
}

// extraPackages is yt-dlp's recommended optional dependency set ("default"
// extras + curl_cffi). These unlock impersonation, websocket-based sites,
// brotli responses, and up-to-date CA certificates.
var extraPackages = []string{
	"curl_cffi", "certifi", "requests", "urllib3", "websockets",
	"brotli", "mutagen", "pycryptodomex",
}

// fileExists is a small helper for our "is it installed?" checks.
func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// runtimeReady reports whether the interpreter, yt-dlp, and the optional
// dependency set are all present.
func runtimeReady() bool {
	py, err := pythonExePath()
	if err != nil {
		return false
	}
	zip, err := ytDlpZipPath()
	if err != nil {
		return false
	}
	marker, err := extrasMarkerPath()
	if err != nil {
		return false
	}
	return fileExists(py) && fileExists(zip) && fileExists(marker)
}

// ytDlpCmd builds the command that runs yt-dlp through our interpreter.
// -I is "isolated mode": ignore the user's PYTHONPATH/site so a broken local
// Python environment can never leak in and break us.
func ytDlpCmd(args ...string) (*exec.Cmd, error) {
	py, err := pythonExePath()
	if err != nil {
		return nil, err
	}
	zip, err := ytDlpZipPath()
	if err != nil {
		return nil, err
	}
	full := append([]string{"-I", zip}, args...)
	return exec.Command(py, full...), nil
}

// --- install flow -----------------------------------------------------------

// DepsStatus reports which managed components are installed, so the frontend
// can offer a separate download button for each.
type DepsStatus struct {
	Python bool `json:"python"`
	YtDlp  bool `json:"ytdlp"`
	Extras bool `json:"extras"`
	// Ffmpeg is true when any usable ffmpeg exists (ours or the system's).
	// FfmpegSkipped is true when the user declined the optional download.
	Ffmpeg        bool `json:"ffmpeg"`
	FfmpegSkipped bool `json:"ffmpegSkipped"`
}

// GetDepsStatus is bound to the frontend setup checklist.
func (a *App) GetDepsStatus() DepsStatus {
	var s DepsStatus
	if py, err := pythonExePath(); err == nil {
		s.Python = fileExists(py)
	}
	if zip, err := ytDlpZipPath(); err == nil {
		s.YtDlp = fileExists(zip)
	}
	if marker, err := extrasMarkerPath(); err == nil {
		s.Extras = fileExists(marker)
	}
	s.Ffmpeg = a.FfmpegAvailable()
	if marker, err := ffmpegSkipMarkerPath(); err == nil {
		s.FfmpegSkipped = fileExists(marker)
	}
	return s
}

// OpenDependenciesFolder reveals the app's managed dependencies folder (the
// interpreter, yt-dlp, optional libraries, and ffmpeg all live here) in the
// OS file manager. Bound to the frontend's "Open dependencies folder" option.
func (a *App) OpenDependenciesFolder() error {
	dir, err := appDir()
	if err != nil {
		return err
	}
	// Create it if it doesn't exist yet so the file manager has a folder to
	// open even before any dependency has been downloaded.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "linux":
		cmd = exec.Command("xdg-open", dir)
	default:
		return fmt.Errorf("opening a folder isn't supported on %s", runtime.GOOS)
	}
	// Start (don't Wait): the file manager keeps running, and explorer.exe is
	// known to return a non-zero exit code even on success.
	return cmd.Start()
}

// InstallPython downloads the standalone Python interpreter. Bound to the
// frontend's per-dependency download button.
func (a *App) InstallPython() error {
	return a.ensurePython()
}

// InstallYtDlpZip downloads the yt-dlp zipapp. Bound to the frontend.
func (a *App) InstallYtDlpZip() error {
	return a.ensureYtDlp()
}

// InstallExtras pip-installs yt-dlp's optional dependencies. Requires the
// Python runtime (pip lives inside it). Bound to the frontend.
func (a *App) InstallExtras() error {
	if py, err := pythonExePath(); err != nil || !fileExists(py) {
		return fmt.Errorf("install the Python runtime first — these libraries are installed using its package manager")
	}
	return a.ensureExtras()
}

// ensureRuntime makes sure everything exists, downloading whatever's missing.
// Progress is reported to the frontend via the same "download:log" /
// "download:progress" events the video download uses, so the existing UI
// shows install progress for free.
func (a *App) ensureRuntime() error {
	if err := a.ensurePython(); err != nil {
		return err
	}
	if err := a.ensureYtDlp(); err != nil {
		return err
	}
	if err := a.ensureExtras(); err != nil {
		return err
	}
	return nil
}

// ensureExtras pip-installs yt-dlp's optional dependencies into our private
// runtime. Without them, only the simplest sites work — curl_cffi in
// particular is what lets yt-dlp impersonate a real browser on sites (Vimeo
// and many others) that refuse plain HTTP clients.
func (a *App) ensureExtras() error {
	marker, err := extrasMarkerPath()
	if err != nil {
		return err
	}
	if fileExists(marker) {
		return nil
	}

	py, err := pythonExePath()
	if err != nil {
		return err
	}

	a.emitLog("Installing site-support libraries (impersonation, certificates)…")

	// The standalone builds ship with pip, but bootstrap it if it's missing.
	if exec.Command(py, "-m", "pip", "--version").Run() != nil {
		if out, eperr := exec.Command(py, "-m", "ensurepip", "--upgrade").CombinedOutput(); eperr != nil {
			return fmt.Errorf("could not set up pip: %s", lastLines(out, 3))
		}
	}

	args := append([]string{"-m", "pip", "install", "--no-warn-script-location", "--quiet"}, extraPackages...)
	if out, piperr := exec.Command(py, args...).CombinedOutput(); piperr != nil {
		return fmt.Errorf("could not install site-support libraries: %s", lastLines(out, 3))
	}

	if err := os.WriteFile(marker, []byte("ok\n"), 0o644); err != nil {
		return err
	}
	a.emitInstallProgress(100)
	return nil
}

// lastLines returns the last n non-empty lines of command output — pip's
// actual error is always at the bottom of its (very long) output.
func lastLines(out []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	keep := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(keep) < n; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			keep = append([]string{s}, keep...)
		}
	}
	return strings.Join(keep, " | ")
}

func (a *App) ensurePython() error {
	py, err := pythonExePath()
	if err != nil {
		return err
	}
	if fileExists(py) {
		return nil
	}

	a.emitLog("Finding the right Python build…")
	url, err := resolvePythonURL()
	if err != nil {
		return fmt.Errorf("could not find a Python build for this system: %w", err)
	}

	a.emitLog("Downloading Python runtime…")
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: server returned %s", resp.Status)
	}

	root, err := pythonRoot()
	if err != nil {
		return err
	}
	// Extract into a staging dir, then atomically swap it into place, so an
	// interrupted download never leaves a half-extracted interpreter behind.
	staging := root + ".tmp"
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}

	// Stream the response straight through gunzip+untar (no giant temp file),
	// wrapping the body so we can report download progress as bytes arrive.
	counted := &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		onTick: func(pct float64) {
			// Treat the download as the first 95% of the bar; extraction is fast.
			a.emitInstallProgress(pct * 0.95)
		},
	}
	if err := extractTarGz(counted, staging); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("could not extract Python runtime: %w", err)
	}

	a.emitLog("Installing Python runtime…")
	_ = os.RemoveAll(root)
	if err := os.Rename(staging, root); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}

	if !fileExists(py) {
		return fmt.Errorf("Python runtime extracted but the interpreter wasn't where expected (%s)", py)
	}
	a.emitInstallProgress(100)
	return nil
}

func (a *App) ensureYtDlp() error {
	zip, err := ytDlpZipPath()
	if err != nil {
		return err
	}
	if fileExists(zip) {
		return nil
	}

	a.emitLog("Downloading yt-dlp…")
	if err := os.MkdirAll(filepath.Dir(zip), 0o755); err != nil {
		return err
	}

	resp, err := http.Get(ytDlpZipURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: server returned %s", resp.Status)
	}

	tmp := zip + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	counted := &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		onTick: func(pct float64) { a.emitInstallProgress(pct) },
	}
	if _, err := io.Copy(out, counted); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	out.Close()

	// Executable bit isn't strictly required (we run it via python), but it
	// keeps the file runnable directly too.
	_ = os.Chmod(tmp, 0o755)
	if err := os.Rename(tmp, zip); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	a.emitInstallProgress(100)
	return nil
}

// --- asset resolution --------------------------------------------------------

// pythonTargetTriple maps Go's OS/arch to python-build-standalone's LLVM triple.
func pythonTargetTriple() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "aarch64-apple-darwin", nil
		}
		return "x86_64-apple-darwin", nil
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "aarch64-unknown-linux-gnu", nil
		}
		return "x86_64-unknown-linux-gnu", nil
	case "windows":
		if runtime.GOARCH == "386" {
			return "i686-pc-windows-msvc", nil
		}
		return "x86_64-pc-windows-msvc", nil
	default:
		return "", fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
}

// resolvePythonURL finds the download URL for the best matching interpreter:
// the newest python-build-standalone release, our platform's triple, the
// "install_only" archive flavor, preferring our chosen Python minor version.
func resolvePythonURL() (string, error) {
	triple, err := pythonTargetTriple()
	if err != nil {
		return "", err
	}

	// 1. Which release is newest?
	var latest struct {
		Tag string `json:"tag"`
	}
	if err := getJSON(pbsLatestURL, &latest); err != nil {
		return "", err
	}
	if latest.Tag == "" {
		return "", fmt.Errorf("could not determine latest Python release")
	}

	// 2. Get that release's numeric id so we can page through its assets.
	var release struct {
		ID int64 `json:"id"`
	}
	apiURL := "https://api.github.com/repos/astral-sh/python-build-standalone/releases/tags/" + latest.Tag
	if err := getJSON(apiURL, &release); err != nil {
		return "", err
	}
	if release.ID == 0 {
		return "", fmt.Errorf("could not read release %s", latest.Tag)
	}

	// 3. Page through the assets (these releases have hundreds, and GitHub caps
	// how many it embeds in the release object, so we use the assets endpoint).
	bestURL := ""
	bestScore := [4]int{-1, -1, -1, -1} // preferred, major, minor, patch
	const maxPages = 30
	for page := 1; page <= maxPages; page++ {
		var assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}
		assetsURL := fmt.Sprintf(
			"https://api.github.com/repos/astral-sh/python-build-standalone/releases/%d/assets?per_page=100&page=%d",
			release.ID, page,
		)
		if err := getJSON(assetsURL, &assets); err != nil {
			return "", err
		}
		if len(assets) == 0 {
			break // no more pages
		}
		for _, asset := range assets {
			name := asset.Name
			if !strings.Contains(name, triple) {
				continue
			}
			if !strings.HasSuffix(name, "-install_only.tar.gz") {
				continue // skips full/stripped/zst builds
			}
			if strings.Contains(name, "freethreaded") {
				continue
			}
			maj, min, patch, ok := parseCPythonVersion(name)
			if !ok {
				continue
			}
			preferred := 0
			if min == preferredPyMinor {
				preferred = 1
			}
			score := [4]int{preferred, maj, min, patch}
			if scoreGreater(score, bestScore) {
				bestScore = score
				bestURL = asset.URL
			}
		}
	}

	if bestURL == "" {
		return "", fmt.Errorf("no install_only Python build found for %s", triple)
	}
	return bestURL, nil
}

// parseCPythonVersion pulls "3.12.8" out of a name like
// "cpython-3.12.8+20260414-aarch64-apple-darwin-install_only.tar.gz".
func parseCPythonVersion(name string) (maj, min, patch int, ok bool) {
	rest, found := strings.CutPrefix(name, "cpython-")
	if !found {
		return 0, 0, 0, false
	}
	ver, _, _ := strings.Cut(rest, "+") // drop "+20260414-..."
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	maj, e1 := strconv.Atoi(parts[0])
	min, e2 := strconv.Atoi(parts[1])
	patch, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, 0, 0, false
	}
	return maj, min, patch, true
}

// scoreGreater compares two [preferred, major, minor, patch] tuples left-to-right.
func scoreGreater(a, b [4]int) bool {
	for i := 0; i < 4; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// getJSON fetches a URL and decodes the JSON body into v.
func getJSON(url string, v interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	// GitHub's API rejects requests that don't send a User-Agent.
	req.Header.Set("User-Agent", "ytpgui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request to %s returned %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// --- extraction --------------------------------------------------------------

// extractTarGz unpacks a gzip-compressed tar stream into dest. It recreates
// directories, regular files (preserving the executable bit), and symlinks —
// the interpreter relies on symlinks like bin/python3 -> python3.12.
func extractTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	cleanDest := filepath.Clean(dest)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(cleanDest, hdr.Name)
		// Guard against path-traversal entries ("../../etc/...").
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			if err := os.Chmod(target, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(filepath.Join(cleanDest, hdr.Linkname), target); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- progress reporting ------------------------------------------------------

// progressReader wraps an io.Reader and reports how far through it has read,
// throttled so we don't flood the frontend with events.
type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	onTick   func(pct float64)
	lastEmit time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	if pr.total > 0 && pr.onTick != nil {
		if time.Since(pr.lastEmit) > 100*time.Millisecond || err == io.EOF {
			pr.lastEmit = time.Now()
			pct := float64(pr.read) / float64(pr.total) * 100
			if pct > 100 {
				pct = 100
			}
			pr.onTick(pct)
		}
	}
	return n, err
}

func (a *App) emitLog(msg string) {
	wailsruntime.EventsEmit(a.ctx, "download:log", msg)
}

func (a *App) emitInstallProgress(pct float64) {
	wailsruntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{Percent: pct})
}

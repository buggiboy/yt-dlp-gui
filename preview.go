package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// --- local preview server ----------------------------------------------------
//
// Many video sites refuse to play embeds when the embedding page doesn't send
// an HTTP Referer header (YouTube's error 153). On macOS, Wails serves the app
// over its custom wails:// scheme, which has no HTTP origin — so an iframe in
// the app can never send a valid Referer. The fix: run a tiny localhost HTTP
// server that serves wrapper pages around the embeds. The app's iframe loads
// the wrapper from http://127.0.0.1, which IS a real HTTP origin, so the
// nested player iframe gets a Referer it accepts.

// videoIDRe matches a YouTube video ID (and nothing else), so the player
// endpoint can't be abused to inject arbitrary content into the page.
var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// wrapperPage is the shared wrapper HTML. %s is replaced with an iframe src
// (already validated + attribute-escaped). %% is a literal % — this string
// goes through fmt.Fprintf.
const wrapperPage = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="referrer" content="strict-origin-when-cross-origin">
<style>html,body{margin:0;height:100%%;background:#000;overflow:hidden}iframe{width:100%%;height:100%%;border:0;display:block}</style>
</head>
<body>
<iframe src="%s"
        allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        referrerpolicy="strict-origin-when-cross-origin"
        allowfullscreen></iframe>
</body>
</html>`

// startPreviewServer binds to a random free port on the loopback interface
// (so nothing outside this machine can reach it) and serves wrapper pages.
func (a *App) startPreviewServer() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	a.previewPort = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()

	// /player/<youtube-id> — fast path for YouTube, no oEmbed lookup needed.
	mux.HandleFunc("/player/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/player/")
		if !videoIDRe.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, wrapperPage, "https://www.youtube-nocookie.com/embed/"+id)
	})

	// /embed?src=<https player url> — generic path for other sites' players
	// (the src comes from the site's own oEmbed response, validated below).
	mux.HandleFunc("/embed", func(w http.ResponseWriter, r *http.Request) {
		src := r.URL.Query().Get("src")
		u, err := url.Parse(src)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, wrapperPage, html.EscapeString(src))
	})

	go http.Serve(ln, mux) //nolint:errcheck — runs for the app's lifetime
	return nil
}

// PreviewPort returns the local preview server's port, or 0 if it failed to
// start (the frontend hides the preview in that case). Bound to the frontend.
func (a *App) PreviewPort() int {
	return a.previewPort
}

// --- generic preview lookup ----------------------------------------------------

// PreviewInfo tells the frontend how to preview a URL. Kind is "embed"
// (URL points at our local wrapper page, ready for an iframe), "thumbnail"
// (Thumbnail is an image URL), or "none".
type PreviewInfo struct {
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
	Title     string `json:"title"`
	Duration  string `json:"duration"`
}

// previewClient is used for all preview-related fetches. Short timeout: this
// runs while the user is typing/pasting, so fail fast rather than hang.
var previewClient = &http.Client{Timeout: 6 * time.Second}

// oembedLinkRe finds <link ...> tags in a page's HTML; we then pick the one
// advertising an oEmbed JSON endpoint.
var (
	linkTagRe = regexp.MustCompile(`(?is)<link[^>]+>`)
	hrefRe    = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']+)["']`)
	iframeRe  = regexp.MustCompile(`(?is)<iframe[^>]+src\s*=\s*["']([^"']+)["']`)
)

// GetPreview resolves the best available preview for any video URL.
// Strategy: ask the site itself via oEmbed (a standard most video sites
// implement) for official embed HTML or a thumbnail; if that fails, fall back
// to yt-dlp's metadata extractor, which knows thumbnails for every site it
// supports. Never returns an error — preview is best-effort decoration.
// Bound to the frontend.
func (a *App) GetPreview(rawURL string) PreviewInfo {
	rawURL = strings.TrimSpace(rawURL)
	pageURL, err := url.Parse(rawURL)
	if err != nil || (pageURL.Scheme != "http" && pageURL.Scheme != "https") {
		return PreviewInfo{Kind: "none"}
	}

	if info, ok := a.previewFromOEmbed(pageURL); ok {
		return info
	}
	if info, ok := a.previewFromYtDlp(rawURL); ok {
		return info
	}
	return PreviewInfo{Kind: "none"}
}

// previewFromOEmbed fetches the page, discovers its oEmbed endpoint, and turns
// the response into a PreviewInfo.
func (a *App) previewFromOEmbed(pageURL *url.URL) (PreviewInfo, bool) {
	body, err := fetchLimited(pageURL.String(), 1<<20) // read at most 1 MiB
	if err != nil {
		return PreviewInfo{}, false
	}

	// Find <link type="application/json+oembed" href="...">.
	var endpoint string
	for _, tag := range linkTagRe.FindAllString(string(body), -1) {
		if !strings.Contains(strings.ToLower(tag), "application/json+oembed") {
			continue
		}
		if m := hrefRe.FindStringSubmatch(tag); m != nil {
			endpoint = html.UnescapeString(m[1])
			break
		}
	}
	if endpoint == "" {
		return PreviewInfo{}, false
	}
	// Resolve relative endpoints against the page URL.
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return PreviewInfo{}, false
	}
	epURL = pageURL.ResolveReference(epURL)

	raw, err := fetchLimited(epURL.String(), 1<<20)
	if err != nil {
		return PreviewInfo{}, false
	}
	var oe struct {
		HTML         string `json:"html"`
		ThumbnailURL string `json:"thumbnail_url"`
		Title        string `json:"title"`
	}
	if err := json.Unmarshal(raw, &oe); err != nil {
		return PreviewInfo{}, false
	}

	// Best case: the provider gave us embed HTML containing an https iframe.
	// We serve that iframe's src through our local wrapper.
	if m := iframeRe.FindStringSubmatch(oe.HTML); m != nil {
		src := html.UnescapeString(m[1])
		if u, err := url.Parse(src); err == nil && u.Scheme == "https" {
			return PreviewInfo{
				Kind:  "embed",
				URL:   fmt.Sprintf("http://127.0.0.1:%d/embed?src=%s", a.previewPort, url.QueryEscape(src)),
				Title: oe.Title,
			}, true
		}
	}
	if oe.ThumbnailURL != "" {
		return PreviewInfo{Kind: "thumbnail", Thumbnail: oe.ThumbnailURL, Title: oe.Title}, true
	}
	return PreviewInfo{}, false
}

// previewFromYtDlp asks yt-dlp for the video's metadata (no download) and uses
// its thumbnail. Slower than oEmbed but works on every site yt-dlp supports.
func (a *App) previewFromYtDlp(rawURL string) (PreviewInfo, bool) {
	if !runtimeReady() {
		return PreviewInfo{}, false
	}
	cmd, err := ytDlpCmd("--dump-json", "--no-playlist", "--playlist-items", "1", "--no-warnings", rawURL)
	if err != nil {
		return PreviewInfo{}, false
	}
	// Metadata extraction should be quick; kill it if a site makes it crawl.
	timer := time.AfterFunc(20*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill() //nolint:errcheck
		}
	})
	defer timer.Stop()

	out, err := cmd.Output()
	if err != nil {
		return PreviewInfo{}, false
	}
	var meta struct {
		Thumbnail      string `json:"thumbnail"`
		Title          string `json:"title"`
		DurationString string `json:"duration_string"`
	}
	if err := json.Unmarshal(out, &meta); err != nil || meta.Thumbnail == "" {
		return PreviewInfo{}, false
	}
	return PreviewInfo{
		Kind:      "thumbnail",
		Thumbnail: meta.Thumbnail,
		Title:     meta.Title,
		Duration:  meta.DurationString,
	}, true
}

// fetchLimited GETs a URL and returns at most limit bytes of the body. The
// User-Agent matters: some sites serve bot-blocking pages to Go's default UA.
func fetchLimited(u string, limit int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
	resp, err := previewClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

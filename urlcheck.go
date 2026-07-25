package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// --- URL validation ----------------------------------------------------------
//
// The frontend does an instant, offline shape check (is this even a web
// address?). This file answers the question that check can't: is there actually
// a downloadable video at the other end? Only yt-dlp knows — it has extractors
// for thousands of sites — so we ask it for metadata and translate whatever it
// says back into a sentence a person can act on.
//
// yt-dlp's diagnostics are written for a terminal, not a GUI: "ERROR:
// [generic] example: Unable to extract data; please report this issue…" is
// accurate but tells the user nothing about what to do. classifyProbeError maps
// the recognisable failures onto short, plain messages instead.

// URLCheckKind is the machine-readable verdict, so the UI can decide how to
// present each case (and which ones warrant a "supported sites" link).
type URLCheckKind = string

const (
	kindOK          URLCheckKind = "ok"          // a video was found
	kindUnsupported URLCheckKind = "unsupported" // yt-dlp has no extractor for this site
	kindNoVideo     URLCheckKind = "novideo"     // supported site (or generic page) with nothing to download
	kindUnavailable URLCheckKind = "unavailable" // the video exists but is deleted, private, or region-locked
	kindAuth        URLCheckKind = "auth"        // needs a login / paid account
	kindNetwork     URLCheckKind = "network"     // we couldn't reach the site at all
	kindUnknown     URLCheckKind = "unknown"     // yt-dlp failed in a way we don't recognise
)

// URLCheck is CheckURL's response. Formats is filled in on success so the
// frontend gets the quality list from the same yt-dlp run — checking the URL
// and listing formats need identical metadata, and spawning yt-dlp twice for
// every paste would double the wait for no benefit.
type URLCheck struct {
	Kind URLCheckKind `json:"kind"`
	// Message is empty when Kind is "ok"; otherwise it's shown to the user
	// verbatim, so it should read as a complete sentence.
	Message string `json:"message"`
	// Blocking reports whether this verdict is certain enough to stop the
	// download. Transient problems (network) are shown but not enforced — the
	// user's connection may well be fine by the time they hit Download.
	Blocking bool `json:"blocking"`
	// Meta carries the video's descriptive metadata, keyed by yt-dlp field name
	// ("title", "uploader", …) so the frontend's filename tokens can look values
	// up directly. It lets the UI show the filename this download will actually
	// produce rather than placeholder samples. Empty unless Kind is "ok".
	Meta    map[string]string `json:"meta"`
	Formats FormatList        `json:"formats"`
}

// probeTimeout caps a metadata lookup. Generous enough for slow extractors that
// walk several pages, short enough that a hung site doesn't wedge the field.
const probeTimeout = 30 * time.Second

// probeURL runs a single metadata-only yt-dlp pass and returns its JSON output.
// On failure the returned error wraps yt-dlp's stderr, which is what the
// classifier reads. Shared by ListFormats and CheckURL so both see exactly the
// same yt-dlp behaviour.
func (a *App) probeURL(rawURL string) ([]byte, error) {
	cmd, err := ytDlpCmd("--dump-json", "--no-playlist", "--playlist-items", "1", "--no-warnings", rawURL)
	if err != nil {
		return nil, err
	}
	timer := time.AfterFunc(probeTimeout, func() {
		if cmd.Process != nil {
			cmd.Process.Kill() //nolint:errcheck
		}
	})
	defer timer.Stop()

	// cmd.Output captures stderr into ExitError.Stderr as long as we leave
	// cmd.Stderr nil — that's where yt-dlp's diagnostics live.
	return cmd.Output()
}

// stderrOf digs yt-dlp's stderr out of an error returned by probeURL.
func stderrOf(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(exitErr.Stderr)
	}
	return err.Error()
}

// probePattern maps a substring of yt-dlp's stderr to the verdict it implies.
// Order matters: the first match wins, so the more specific phrases come first
// (a private video also mentions "unavailable", for instance).
type probePattern struct {
	needle  string
	kind    URLCheckKind
	message string
}

var probePatterns = []probePattern{
	// --- site not supported at all ---
	{"unsupported url", kindUnsupported,
		"This site isn't supported."},

	// --- needs credentials ---
	{"private video", kindAuth,
		"This video is private."},
	{"members-only", kindAuth,
		"This video is for channel members only."},
	{"join this channel", kindAuth,
		"This video is for channel members only."},
	{"sign in to confirm your age", kindAuth,
		"This video is age-restricted and needs a signed-in account."},
	{"age-restricted", kindAuth,
		"This video is age-restricted and needs a signed-in account."},
	// Must precede the generic "sign in" rule below. This is YouTube throttling
	// the client, not a real permission problem — it usually clears on its own,
	// so it's classified as transient and left non-blocking.
	{"not a bot", kindNetwork,
		"YouTube is rate-limiting requests right now. Try again in a moment."},
	{"sign in", kindAuth,
		"This video requires a signed-in account."},
	{"login required", kindAuth,
		"This video requires a signed-in account."},
	{"paid members", kindAuth,
		"This video is behind a paywall."},

	// --- exists, but not for us ---
	{"not available in your country", kindUnavailable,
		"This video isn't available in your region."},
	{"blocked it in your country", kindUnavailable,
		"This video isn't available in your region."},
	{"geo restricted", kindUnavailable,
		"This video isn't available in your region."},
	{"video unavailable", kindUnavailable,
		"This video is unavailable — it may have been removed."},
	{"has been removed", kindUnavailable,
		"This video is unavailable — it may have been removed."},
	{"account associated with this video has been terminated", kindUnavailable,
		"This video is unavailable — the account was terminated."},
	{"this video has been removed", kindUnavailable,
		"This video is unavailable — it may have been removed."},
	{"http error 404", kindUnavailable,
		"That page doesn't exist (404)."},
	{"http error 410", kindUnavailable,
		"This video is unavailable — it may have been removed."},
	{"http error 403", kindUnavailable,
		"That site refused the request (403)."},

	// --- reachable page, nothing to download ---
	{"no video formats found", kindNoVideo,
		"No video found at this link."},
	{"there's no video", kindNoVideo,
		"No video found at this link."},
	{"unable to extract", kindNoVideo,
		"No video found at this link."},
	{"unable to recognize", kindNoVideo,
		"No video found at this link."},
	{"no media found", kindNoVideo,
		"No video found at this link."},
	{"does not have a video", kindNoVideo,
		"No video found at this link."},

	// --- we never got there ---
	{"unable to download webpage", kindNetwork,
		"Couldn't reach that site. Check your connection and try again."},
	{"name or service not known", kindNetwork,
		"Couldn't find that site — check the address for typos."},
	{"nodename nor servname", kindNetwork,
		"Couldn't find that site — check the address for typos."},
	{"failed to resolve", kindNetwork,
		"Couldn't find that site — check the address for typos."},
	{"getaddrinfo", kindNetwork,
		"Couldn't find that site — check the address for typos."},
	{"connection refused", kindNetwork,
		"Couldn't reach that site. Check your connection and try again."},
	{"connection reset", kindNetwork,
		"Couldn't reach that site. Check your connection and try again."},
	{"timed out", kindNetwork,
		"That site took too long to respond."},
	{"read timed out", kindNetwork,
		"That site took too long to respond."},
	{"certificate verify failed", kindNetwork,
		"Couldn't establish a secure connection to that site."},
}

// classifyProbeError turns yt-dlp's stderr into a verdict. Everything is
// lowercased first so the patterns don't have to care about yt-dlp's mix of
// sentence casing across extractors.
func classifyProbeError(stderr string) (URLCheckKind, string) {
	low := strings.ToLower(stderr)
	for _, p := range probePatterns {
		if strings.Contains(low, p.needle) {
			return p.kind, p.message
		}
	}
	return kindUnknown, "Couldn't read a video from this link."
}

// blockingKinds are the verdicts we're confident enough about to disable the
// Download button. Network problems and unrecognised failures are shown as a
// heads-up but still let the user try — we'd rather occasionally show a stale
// warning than refuse a download that would have worked.
var blockingKinds = map[URLCheckKind]bool{
	kindUnsupported: true,
	kindNoVideo:     true,
	kindUnavailable: true,
	kindAuth:        true,
}

// CheckURL asks yt-dlp whether there's a downloadable video at rawURL and, on
// success, returns the available formats from the same lookup. It never returns
// an error: a failed check is a result the UI displays, not an exception.
// Bound to the frontend, called on a debounce as the user types.
func (a *App) CheckURL(rawURL string) URLCheck {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return URLCheck{Kind: kindOK}
	}
	if !runtimeReady() {
		// The setup panel is already telling the user to install yt-dlp; don't
		// pile a second, misleading complaint onto the URL field.
		return URLCheck{Kind: kindOK}
	}

	out, err := a.probeURL(rawURL)
	if err != nil {
		kind, msg := classifyProbeError(stderrOf(err))
		return URLCheck{Kind: kind, Message: msg, Blocking: blockingKinds[kind]}
	}

	meta, perr := parseProbeFormats(out)
	if perr != nil {
		return URLCheck{
			Kind:     kindNoVideo,
			Message:  "No video found at this link.",
			Blocking: true,
		}
	}
	return URLCheck{Kind: kindOK, Meta: meta.meta, Formats: meta.list}
}

// probeMeta is parseProbeFormats' result: the pieces of yt-dlp's metadata dump
// the URL check cares about.
type probeMeta struct {
	meta map[string]string
	list FormatList
}

// parseProbeFormats reads a yt-dlp --dump-json blob into the distinct video
// heights (high→low, each with the container of the best format at that height)
// plus the best audio-only container, and the descriptive metadata the filename
// preview needs. Returns an error when the JSON is unreadable or advertises no
// video at all.
func parseProbeFormats(out []byte) (probeMeta, error) {
	var m probeMeta
	list, err := decodeFormatList(out)
	if err != nil {
		return m, err
	}
	if len(list.Videos) == 0 && list.AudioExt == "" {
		return m, fmt.Errorf("no downloadable formats")
	}
	m.list = list
	m.meta = decodePreviewMeta(out)
	return m, nil
}

// decodePreviewMeta pulls the descriptive fields out of a --dump-json blob and
// returns them keyed by yt-dlp field name, ready for the frontend's filename
// tokens.
//
// Deliberately omitted: resolution, height, and fps. Those describe whichever
// format yt-dlp settles on at download time, which depends on the user's
// quality choice and isn't decided yet — echoing the probe's numbers would
// state a specific, confidently wrong answer, which is worse than an obvious
// placeholder. Playlist fields are omitted for the same reason: downloads pass
// --no-playlist, so there is no playlist context.
//
// Never errors: a missing or unparseable field just doesn't appear in the map,
// and the preview falls back to its sample value for that token.
func decodePreviewMeta(out []byte) map[string]string {
	var raw struct {
		Title          string `json:"title"`
		ID             string `json:"id"`
		Uploader       string `json:"uploader"`
		Channel        string `json:"channel"`
		ChannelID      string `json:"channel_id"`
		UploadDate     string `json:"upload_date"`
		ReleaseDate    string `json:"release_date"`
		DurationString string `json:"duration_string"`
		ViewCount      int64  `json:"view_count"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	meta := map[string]string{}
	for field, value := range map[string]string{
		"title":           raw.Title,
		"id":              raw.ID,
		"uploader":        raw.Uploader,
		"channel":         raw.Channel,
		"channel_id":      raw.ChannelID,
		"upload_date":     raw.UploadDate,
		"release_date":    raw.ReleaseDate,
		"duration_string": raw.DurationString,
	} {
		// Blank means the extractor didn't supply it; leave it out so the
		// preview's sample fills the gap instead of showing an empty token.
		if value != "" {
			meta[field] = value
		}
	}
	if raw.ViewCount > 0 {
		meta["view_count"] = strconv.FormatInt(raw.ViewCount, 10)
	}
	return meta
}

package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Argument assembly for yt-dlp, kept in one place.
//
// This file is the single source of truth for "which flags do these options
// turn into". Both the download that actually runs (DownloadVideo, ytdlp.go)
// and the command preview shown in the UI (PreviewCommand, below) go through
// buildArgs, so the preview cannot describe a command the app wouldn't run.
//
// It deliberately contains no process handling and no I/O beyond resolving the
// default Downloads folder: everything here is a pure function of the options
// plus an explicit argEnv, which is what makes it testable without a real
// ffmpeg or Python install.

// DownloadOptions bundles everything the Download button sends. A struct
// (rather than positional string params) so new options can be added without
// touching every call site — the frontend just gains another field.
//
// This is the contract with the frontend: add new download options here, and
// turn them into flags in buildArgs below.
type DownloadOptions struct {
	URL                 string   `json:"url"`
	Start               string   `json:"start"`               // clip start ("" = from beginning)
	End                 string   `json:"end"`                 // clip end ("" = to the end)
	Playlist            bool     `json:"playlist"`            // download every video in the playlist rather than just this one
	Quality             string   `json:"quality"`             // "best", "audio", or a max height like "1080"
	AudioFormat         string   `json:"audioFormat"`         // "", "mp3", "m4a", "opus", "flac" (audio-only)
	Folder              string   `json:"folder"`              // save location ("" = Downloads)
	Subtitles           bool     `json:"subtitles"`           // download and embed subtitles
	SubLangs            string   `json:"subLangs"`            // subtitle language codes ("en", "en,fr", "all")
	EmbedMeta           bool     `json:"embedMeta"`           // embed metadata + thumbnail + chapters
	SponsorBlock        []string `json:"sponsorBlock"`        // SponsorBlock categories to remove (empty = off)
	RateLimit           string   `json:"rateLimit"`           // max download rate for --limit-rate (e.g. "5M"; "" = unlimited)
	ConcurrentFragments int      `json:"concurrentFragments"` // parallel fragment downloads (--concurrent-fragments N; 0 = off, yt-dlp default of 1)
	ExtraArgs           string   `json:"extraArgs"`           // raw extra yt-dlp flags (power-user escape hatch)
	Outtmpl             string   `json:"outtmpl"`             // filename template, no folder ("" = default %(title)s.%(ext)s)
	NameArgs            []string `json:"nameArgs"`            // naming flags from the format builder (--replace-in-metadata, --restrict-filenames, --trim-filenames)
}

// argEnv carries the parts of the outside world that argument assembly depends
// on. Passing them in (rather than calling FfmpegAvailable and friends inline)
// keeps buildArgs a pure function of its inputs.
type argEnv struct {
	// ffmpegAvailable gates the options that can't work without it: section
	// clipping and audio extraction/conversion.
	ffmpegAvailable bool
	// dir is the already-resolved save folder. Never empty by the time
	// buildArgs sees it — see resolveFolder.
	dir string
	// lenient makes the builder skip an option it can't make sense of instead
	// of failing on it. The preview re-renders on every keystroke, so a
	// half-typed timestamp ("1:") or an unclosed quote in Extra arguments must
	// not blank the whole command line; a real download, by contrast, has to
	// refuse them loudly. Nothing else differs between the two modes.
	lenient bool
}

// resolveFolder turns the options' folder into the directory the download will
// write to, substituting the user's Downloads folder for an empty choice.
// Deliberately does not check that the directory exists — DownloadVideo does
// that separately, because the preview shouldn't stat the filesystem on every
// keystroke (nor blank itself over a folder that's merely been unplugged).
func resolveFolder(folder string) (string, error) {
	if dir := strings.TrimSpace(folder); dir != "" {
		return dir, nil
	}
	return downloadsDir()
}

// buildArgs assembles the yt-dlp arguments for a download, ending with the URL.
//
// It emits only the flags that correspond to a user's choices. The internal
// plumbing the real run needs — --newline, --progress-template, and
// --ffmpeg-location — is prepended by DownloadVideo, so it stays out of the
// preview where it would only be noise.
//
// Ordering is deliberate and load-bearing at exactly two points: NameArgs come
// before ExtraArgs, and ExtraArgs come last before the URL, so that a
// power-user flag overrides the matching flag the UI set wherever yt-dlp
// honours the later occurrence.
func buildArgs(opts DownloadOptions, env argEnv) ([]string, error) {
	// Output template: the filename pattern from the format builder (falling
	// back to "<video title>.<extension>"), saved into the target folder. The
	// template may contain "/" to create subfolders, which filepath.Join
	// preserves.
	name := strings.TrimSpace(opts.Outtmpl)
	if name == "" {
		name = "%(title)s.%(ext)s"
	}
	args := []string{"-o", filepath.Join(env.dir, name)}

	// Playlist scope. A link copied from inside a playlist carries "&list=…",
	// which yt-dlp expands into every video in it by default — so the default
	// here is the opposite, matching what the URL check probed and what the
	// filename preview promised. Opting in is an explicit checkbox the UI only
	// offers when the URL looks like a playlist.
	if opts.Playlist {
		args = append(args, "--yes-playlist")
	} else {
		args = append(args, "--no-playlist")
	}

	qArgs, err := qualityArgs(opts.Quality, opts.AudioFormat, env.ffmpegAvailable)
	if err != nil && !env.lenient {
		return nil, err
	}
	args = append(args, qArgs...)

	clipArgs, err := sectionArgs(opts.Start, opts.End, env.ffmpegAvailable)
	if err != nil && !env.lenient {
		return nil, err
	}
	args = append(args, clipArgs...)

	// Subtitles: prefer manually-uploaded subs, fall back to auto-generated.
	// --embed-subs muxes them into the container (requires ffmpeg for mp4).
	if opts.Subtitles {
		langs := strings.TrimSpace(opts.SubLangs)
		if langs == "" {
			langs = "en"
		}
		args = append(args,
			"--write-subs",
			"--write-auto-subs",
			"--sub-langs", langs,
			"--embed-subs",
		)
	}

	// Embed metadata, thumbnail, and chapter markers into the output file.
	// Thumbnail embedding requires ffmpeg; yt-dlp will skip it gracefully if
	// ffmpeg isn't available.
	if opts.EmbedMeta {
		args = append(args,
			"--embed-metadata",
			"--embed-thumbnail",
			"--embed-chapters",
		)
	}

	// SponsorBlock: remove the selected segment categories. yt-dlp calls
	// SponsorBlock's API and cuts the matched segments via ffmpeg.
	if len(opts.SponsorBlock) > 0 {
		args = append(args, "--sponsorblock-remove", strings.Join(opts.SponsorBlock, ","))
	}

	// Speed limit: cap the download rate so it doesn't saturate the connection.
	// yt-dlp accepts a number with an optional unit suffix (e.g. "500K", "5M").
	if rate := strings.TrimSpace(opts.RateLimit); rate != "" {
		args = append(args, "--limit-rate", rate)
	}

	// Parallel fragments: fetch multiple fragments at once. A big speedup on
	// sites that serve fragmented (DASH/HLS) media. 0 means leave yt-dlp at its
	// default of 1 (sequential).
	if opts.ConcurrentFragments > 0 {
		args = append(args, "--concurrent-fragments", strconv.Itoa(opts.ConcurrentFragments))
	}

	// Naming flags from the filename-format builder (--replace-in-metadata,
	// --restrict-filenames, --trim-filenames). Already tokenized by the
	// frontend as discrete argv entries, so they're appended verbatim — no
	// shell parsing.
	args = append(args, opts.NameArgs...)

	// Extra arguments: a raw escape hatch for power users who need a flag the
	// UI doesn't expose. Parsed shell-style so quoted values survive.
	if extra := strings.TrimSpace(opts.ExtraArgs); extra != "" {
		extraArgs, err := splitArgs(extra)
		if err != nil {
			if !env.lenient {
				return nil, err
			}
			// Mid-edit: an unbalanced quote is what a half-typed argument looks
			// like. Drop the field from this render; it reappears intact once
			// the quote is closed.
			extraArgs = nil
		}
		args = append(args, extraArgs...)
	}

	return append(args, strings.TrimSpace(opts.URL)), nil
}

// sectionArgs builds the --download-sections spec for a clip range, or nothing
// when neither end is set. Both timestamps are validated here so an invalid one
// is reported before yt-dlp is ever started.
func sectionArgs(start, end string, ffmpegAvailable bool) ([]string, error) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" && end == "" {
		return nil, nil
	}
	if !ffmpegAvailable {
		return nil, fmt.Errorf("downloading a section requires ffmpeg, which wasn't found on your system — install it (e.g. from ffmpeg.org) or clear the start/stop fields")
	}

	startSec, endSec := 0.0, -1.0
	from, to := "0", "inf"
	var err error
	if start != "" {
		if startSec, err = parseTimestamp(start); err != nil {
			return nil, fmt.Errorf("start time: %w", err)
		}
		from = start
	}
	if end != "" {
		if endSec, err = parseTimestamp(end); err != nil {
			return nil, fmt.Errorf("stop time: %w", err)
		}
		to = end
	}
	if endSec >= 0 && endSec <= startSec {
		return nil, fmt.Errorf("stop time must be after start time")
	}

	return []string{
		"--download-sections", fmt.Sprintf("*%s-%s", from, to),
		// Re-encode just around the cut points so the clip starts and ends
		// exactly where requested instead of snapping to the nearest keyframe.
		"--force-keyframes-at-cuts",
	}, nil
}

// splitArgs splits a raw flag string into individual arguments the way a shell
// would, honoring single and double quotes (and backslash escapes outside
// single quotes) so users can pass values containing spaces — e.g.
//
//	-o '%(title)s [%(id)s].%(ext)s'
//
// It performs no variable expansion, globbing, or other shell processing: it
// only tokenises. An unbalanced quote or dangling backslash is reported as an
// error so the user gets clear feedback rather than a confusing yt-dlp failure.
func splitArgs(s string) ([]string, error) {
	var (
		args    []string
		cur     strings.Builder
		inArg   bool
		quote   rune // 0, '\'' or '"'
		escaped bool
	)
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			inArg = true
			escaped = false
		case r == '\\' && quote != '\'':
			// Backslash escapes the next char everywhere except inside
			// single quotes (matching POSIX shell behaviour).
			escaped = true
			inArg = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case unicode.IsSpace(r):
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unbalanced %c quote in extra arguments", quote)
	}
	if escaped {
		return nil, fmt.Errorf("dangling backslash at end of extra arguments")
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}

// --- command preview ---------------------------------------------------------

// shellSafeRe matches arguments that a shell would pass through untouched, so
// the preview only adds quotes where a user typing the command would have to.
var shellSafeRe = regexp.MustCompile(`^[A-Za-z0-9_\-./:%@,=]+$`)

// shellQuote renders one argument as it would have to be typed at a POSIX
// shell. Display only — the real download passes argv directly to the process
// and never goes near a shell.
func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if shellSafeRe.MatchString(arg) {
		return arg
	}
	// Single quotes protect everything except a single quote itself, which has
	// to be closed, escaped, and reopened.
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// PreviewCommand renders the yt-dlp command the given options would run, as a
// shell string the user could paste into a terminal themselves.
//
// It shares buildArgs with DownloadVideo, which is the whole point: the preview
// is generated from the same code that assembles the real argv, so the two
// cannot drift apart. What it leaves out is only plumbing the user didn't ask
// for (--newline, --progress-template, --ffmpeg-location).
//
// Never returns an error. It runs on every keystroke against options that are
// still being edited, so anything it can't make sense of is simply left out of
// this render (see argEnv.lenient). Returns "" when there is no URL yet.
// Bound to the frontend.
func (a *App) PreviewCommand(opts DownloadOptions) string {
	if strings.TrimSpace(opts.URL) == "" {
		return ""
	}
	dir, err := resolveFolder(opts.Folder)
	if err != nil {
		dir = "" // no home directory: show the bare filename rather than nothing
	}
	args, err := buildArgs(opts, argEnv{
		ffmpegAvailable: a.FfmpegAvailable(),
		dir:             dir,
		lenient:         true,
	})
	if err != nil {
		// Unreachable in lenient mode, but a preview that swallowed a real
		// failure would be worse than one that goes quiet.
		return ""
	}

	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return "yt-dlp " + strings.Join(quoted, " ")
}

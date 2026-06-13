# yt-dlp-gui

A simple, clean desktop app for downloading videos with [yt-dlp](https://github.com/yt-dlp/yt-dlp) — no terminal required.

Paste a link, pick what you want, hit download. yt-dlp-gui wraps yt-dlp's most useful features in a tidy interface, and it manages its own copies of Python, yt-dlp, and ffmpeg so you don't have to install anything yourself.

> Built with [Wails](https://wails.io), a Go backend, and a Svelte 5 + TypeScript frontend. Runs on macOS and Windows (linux verification coming soon).

## Getting started

1. Grab the latest build from the [Releases](https://github.com/buggiboy/yt-dlp-gui/releases) page (or [build it yourself](#building-from-source)).
2. Open the app. On first launch it walks you through setup that downloads the app's dependencies — a standalone Python runtime, the latest yt-dlp, and (optionally) ffmpeg. Everything lives in the app's own folder; nothing touches your system Python.
3. Paste a video URL into the field

### macOS users — first launch

The macOS build is not yet code-signed or notarized by Apple, so Gatekeeper will block it the first time. This is expected. To open it:

- **Recommended:** unzip the download, then **right-click (or Control-click) `ytpgui.app` → Open**, and confirm **Open** in the dialog. You only need to do this once; afterward it launches normally.
- If you double-clicked first and macOS said the app is *"damaged"* or *"can't be opened"*, clear the quarantine flag in Terminal and reopen:

  ```bash
  xattr -dr com.apple.quarantine /path/to/ytpgui.app
  ```

  (Replace the path, or drag the app onto the Terminal window to fill it in.)

## Using the app

**Paste a URL.** Drop in a link and ytpgui fetches a preview — title and thumbnail — so you know you've got the right video before downloading.

**Pick your quality.** Choose the best available, cap it at a resolution (1080p, 720p, and so on), or grab audio only. For audio-only you can convert to `mp3`, `m4a`, `opus`, or `flac`.

**Clip a section** *(optional)*. Only want part of the video? Set a start and/or stop time — `90`, `1:30`, or `1:02:30` all work — and ytpgui downloads just that slice, cut cleanly at the timestamps you asked for. (Requires ffmpeg.)

**Advanced options** *(optional)*. Tucked away until you need them:

- **Subtitles** — download and embed subtitles, with language codes of your choosing (`en`, `en,fr`, or `all`).
- **Metadata** — embed the title, thumbnail, and chapter markers into the file.
- **SponsorBlock** — automatically skip sponsor segments, intros, and other categories you'd rather not keep.

**Choose where it saves.** Files land in your Downloads folder by default, or point it anywhere you like.

**Watch it go.** A live progress bar shows percentage, speed, and ETA while the download runs.

## Building from source

You'll need [Go](https://go.dev/dl/) 1.23+, [Node.js](https://nodejs.org), and the [Wails CLI](https://wails.io/docs/gettingstarted/installation).

```bash
# clone and enter the project
git clone https://github.com/buggiboy/yt-dlp-gui.git
cd yt-dlp-gui/ytpgui

# live development with hot reload
wails dev

# production build → build/bin/
wails build
```

`wails dev` runs a Vite dev server with fast frontend hot reload. A browser dev server is also available at `http://localhost:34115` if you want to call the Go methods from devtools. `wails build` produces a redistributable package in `build/bin`.

## How it works

yt-dlp-gui doesn't ask you to install yt-dlp, Python, or ffmpeg by hand. On first run it provides an interface to download a self-contained Python interpreter, the yt-dlp zipapp, supporting python libraries, and ffmpeg — each as a separate, resumable step you can see in the setup panel. The Go backend shells out to yt-dlp, streams its progress back to the frontend over Wails events, and translates your UI choices into the right yt-dlp arguments. ffmpeg is optional but unlocks audio conversion, section clipping, and embedding.

## Acknowledgements

All the heavy lifting is done by [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org). yt-dlp-gui just gives them an easy interface.

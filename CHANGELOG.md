# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- **Cancel a download** — the Download button becomes Cancel while a download runs, stopping yt-dlp and any ffmpeg it spawned. Partial `.part` files are kept, so re-running resumes. A download in progress is also stopped when the app closes, instead of being orphaned.
- **Update yt-dlp** — Settings → Dependencies now shows the installed version and a Check for updates button. The check asks GitHub what the current release is without downloading anything; if there's a newer one it names the version, offers to install it, and links the changelog (the release's own notes when you're one behind, a compare view covering every release in between when you're further back). Previously the copy downloaded on first run was kept forever, which breaks as sites change.
- **Reset all settings** — a new Settings → General tab restores every saved setting to its default, for when a persisted value (most likely a bad Extra argument) is causing trouble.
- **Whole-playlist downloads** — when the URL points into a playlist, a "Download the whole playlist" checkbox appears under the address. Off by default (only the video you pasted is fetched), and it resets whenever the URL moves to a different playlist.

### Changed

- **Settings are remembered between sessions.** The download folder, subtitle options, metadata embedding, SponsorBlock categories, audio format, and Extra arguments now persist alongside the filename format and speed settings that already did. Quality and clip times deliberately don't — quality gets snapped to whatever a given video offers (so a saved value would drift downwards), and a stale clip range would silently truncate the next download.
- Saved Extra arguments are flagged with a badge on the collapsed "Advanced options" header, so a persisted flag can't sit there invisibly breaking every download.
- **The app is now called yt-dlp-gui throughout**, replacing the internal `ytpgui` name: the window title, the macOS `.app` bundle, the Windows executable, and the release artifacts (`yt-dlp-gui-mac-universal.zip`, `yt-dlp-gui-windows-amd64.exe`, `yt-dlp-gui-linux-amd64`). The per-user data folder moves from `ytpgui` to `yt-dlp-gui` too — existing installs are migrated automatically on first launch, keeping the downloaded Python runtime, yt-dlp, ffmpeg and settings.
- The speed limit is now a whole number plus a KB/s / MB/s unit dropdown, replacing the MB/s-only decimal field.

### Fixed

- The "Saves as" line under the URL showed the placeholder sample title ("Never Gonna Give You Up…") instead of the real video's. The URL check already fetched the actual metadata and discarded it; it's now carried through, so the filename preview — along with the Advanced-options summary and the filename format builder — reflects the video you actually pasted. No extra yt-dlp run.
- A link copied from inside a playlist (`…&list=…`) downloaded the entire playlist, despite the URL check and filename preview both describing a single video. Downloads now pass `--no-playlist` unless the playlist checkbox is ticked.
- The speed limit field cleared itself while typing a decimal, and rewrote the settings file on every keystroke.
- A download could hang forever if yt-dlp emitted a line over 64 KB, with no way to recover short of quitting.
- A video needing a sign-in no longer disables the Download button when cookies are supplied in Extra arguments — the check runs anonymously, so it can't know what the account can see. When no cookies are set, the message now says how to add them.

## [1.1.0] - 2026-06-13

### Added

- **Settings modal** — configure download rate limit (MB/s) and concurrent fragment downloads (4/8/16) via a tabbed settings panel. Settings persist between sessions.
- **Filename format modal** — drag-and-drop filename template builder with token fields (title, uploader, date, ID, etc.), live preview, named presets (YouTube-style, minimal, archive), cleanup rules (whitespace, special characters, Windows-safe), and a custom rules editor.

### Changed

- Styling improvements.
- Internal logic refactors.

### Changed

- App window title updated to "yt-dlp-gui - Youtube Downloader".
- App screenshot updated to reflect the new UI.

## [1.0.0] - Initial release

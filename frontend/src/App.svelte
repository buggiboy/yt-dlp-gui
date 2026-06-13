<script lang="ts">
  import { onMount } from 'svelte'
  import {
    DownloadVideo,
    GetDepsStatus,
    GetPreview,
    InstallExtras,
    InstallFfmpeg,
    InstallPython,
    InstallYtDlpZip,
    ListFormats,
    SkipFfmpeg,
    PreviewPort,
    YtDlpVersion,
  } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'

  // --- Svelte 5 runes ---
  // $state() makes a variable reactive. Reassigning it re-renders the UI,
  // similar to a useState value in React — but you mutate it directly,
  // there's no setter function.
  let url = $state('')
  let version = $state('')
  let status = $state('')
  let busy = $state(false)

  // Per-dependency install state. The app downloads three components, each
  // with its own button and confirmation modal so the user knows exactly
  // what's being fetched and from where.
  type DepKey = 'python' | 'ytdlp' | 'extras' | 'ffmpeg'
  const depKeys: DepKey[] = ['python', 'ytdlp', 'extras', 'ffmpeg']
  let deps = $state({
    python: false,
    ytdlp: false,
    extras: false,
    ffmpeg: false,
    ffmpegSkipped: false,
  })
  // The core trio is required to download anything; ffmpeg is optional, so it
  // gates only the setup panel (until installed or skipped), not downloading.
  let installed = $derived(deps.python && deps.ytdlp && deps.extras)
  let setupDone = $derived(installed && (deps.ffmpeg || deps.ffmpegSkipped))
  // Which dependency's confirmation modal is open (null = none).
  let confirmTarget: DepKey | null = $state(null)

  const depInfo: Record<
    DepKey,
    {
      name: string
      short: string
      size: string
      source: string
      blurb: string
      optional?: boolean
    }
  > = {
    python: {
      name: 'Python runtime',
      short: 'Standalone bundled copy of Python to run yt-dlp (used instead of yt-dlp binary to improve startup speed)',
      size: '~45 MB',
      source: 'github.com/astral-sh/python-build-standalone',
      blurb:
        'yt-dlp is a Python program, so the app needs a Python interpreter to run it. ' +
        'This downloads a standalone Python build into the app’s own folder — it ' +
        'won’t touch or conflict with any Python already on your system. It comes from ' +
        'the official GitHub releases of python-build-standalone, a project maintained by Astral.',
    },
    ytdlp: {
      name: 'yt-dlp',
      short: 'The actual open-source tool from its official github repository',
      size: '~3 MB',
      source: 'github.com/yt-dlp/yt-dlp',
      blurb:
        'yt-dlp is the open-source program that does the actual video downloading, ' +
        'with support for thousands of sites. This downloads the official release ' +
        'directly from the yt-dlp project’s GitHub releases page.',
    },
    extras: {
      name: 'Site-support python libraries',
      short: 'Needed by many sites (browser impersonation, certificates)',
      size: '~10 MB',
      source: 'pypi.org (official Python package index)',
      blurb:
        'A set of optional libraries that yt-dlp recommends: curl_cffi (lets yt-dlp ' +
        'identify itself like a normal browser, which sites like Vimeo require), plus ' +
        'certificates, websocket, and decryption helpers. Installed from PyPI, the ' +
        'official Python package index, using the standalone Python runtime you already downloaded above.',
    },
    ffmpeg: {
      name: 'ffmpeg',
      short: 'Optional — only needed to download a section of a video or extract audio',
      size: '~30–90 MB',
      source: 'ffmpeg.martin-riedl.de (macOS/Linux) · github.com/yt-dlp/FFmpeg-Builds (Windows)',
      blurb:
        'ffmpeg is the open-source tool yt-dlp uses to cut and convert media. ' +
        'You only need it for the "download only a section" feature and for ' +
        'audio-only extraction — full-video downloads work fine without it. ' +
        'On macOS and Linux this downloads signed static builds from Martin ' +
        'Riedl’s ffmpeg build server; on Windows it uses the builds the ' +
        'yt-dlp project itself maintains. You can skip this and install it later.',
      optional: true,
    },
  }

  async function skipFfmpeg() {
    try {
      await SkipFfmpeg()
    } finally {
      await refreshDeps()
    }
  }

  // --- video quality ---
  // quality is what we pass to Go: 'best', 'audio', or a max height as a
  // string ('1080'). The dropdown starts with generic presets and narrows to
  // the video's real resolutions once ListFormats responds.
  type VideoFormat = { height: number; ext: string }
  const presetHeights: VideoFormat[] = [2160, 1440, 1080, 720, 480].map((h) => ({
    height: h,
    ext: '', // unknown until the real format list loads
  }))
  let quality = $state('best')
  // null = nothing fetched yet (show presets); [] = fetch returned nothing
  // useful (also show presets); otherwise the video's actual formats.
  let availableFormats = $state<VideoFormat[] | null>(null)
  // Container ext of the video's best audio-only format ('' until loaded).
  let audioExt = $state('')
  let formatsLoading = $state(false)
  let formatOptions = $derived(
    availableFormats && availableFormats.length > 0 ? availableFormats : presetHeights
  )

  function qualityLabel(f: VideoFormat): string {
    let label = `${f.height}p`
    if (f.height >= 4320) label += ' (8K)'
    else if (f.height >= 2160) label += ' (4K)'
    if (f.ext) label += ` · ${f.ext}`
    return label
  }

  // Section download (clip) state. The section card is always visible; it's
  // active whenever either time field has a value, and disabled entirely
  // until ffmpeg is available.
  let clipStart = $state('')
  let clipEnd = $state('')
  let ffmpegOk = $state(true)

  // Live download progress, driven by Wails events emitted from Go.
  let percent = $state(0)
  let speed = $state('')
  let eta = $state('')
  let step = $state('')

  // $derived() computes a value from other reactive state and stays in sync
  // automatically — like useMemo in React, but with no dependency array:
  // Svelte tracks what you read inside it.
  const timeRe = /^(\d+(\.\d+)?|(\d+:)?[0-5]?\d:[0-5]\d(\.\d+)?)$/

  // Pull the 11-character video ID out of any common YouTube URL shape.
  // Returns null for anything that isn't recognizably a YouTube video.
  function extractVideoId(raw: string): string | null {
    let parsed: URL
    try {
      parsed = new URL(raw.trim())
    } catch {
      return null // not a complete URL yet (user may still be typing)
    }
    const host = parsed.hostname.replace(/^(www\.|m\.|music\.)/, '')
    let id: string | null = null
    if (host === 'youtu.be') {
      id = parsed.pathname.split('/')[1] ?? null
    } else if (host === 'youtube.com' || host === 'youtube-nocookie.com') {
      if (parsed.pathname === '/watch') {
        id = parsed.searchParams.get('v')
      } else {
        const m = parsed.pathname.match(/^\/(embed|shorts|live|v)\/([^/?]+)/)
        id = m ? m[2] : null
      }
    }
    return id && /^[\w-]{11}$/.test(id) ? id : null
  }

  // Recomputes whenever `url` changes; the preview iframe only remounts when
  // the extracted ID actually changes, not on every keystroke.
  let videoId = $derived(extractVideoId(url))

  // Port of the Go-side localhost server that wraps the YouTube embed.
  // YouTube requires an HTTP Referer from the embedding page (error 153
  // otherwise), and the app's own wails:// origin can't send one — but a
  // wrapper page served from http://127.0.0.1 can.
  let previewPort = $state(0)
  let previewUrl = $derived(
    videoId && previewPort ? `http://127.0.0.1:${previewPort}/player/${videoId}` : null
  )

  // Generic preview for non-YouTube sites, resolved by the Go side
  // (oEmbed first, yt-dlp metadata as fallback).
  type Preview = { kind: string; url: string; thumbnail: string; title: string; duration: string }
  let preview: Preview | null = $state(null)
  let previewLoading = $state(false)

  // $effect runs after render whenever state it reads changes — like useEffect,
  // but the dependencies (url, videoId, previewPort) are tracked automatically.
  // The returned function is the cleanup, which runs before each re-run; we use
  // it to debounce (cancel the pending lookup while the user is still typing)
  // and to ignore stale in-flight responses.
  $effect(() => {
    const raw = url.trim()
    preview = null
    if (videoId || !previewPort || !/^https?:\/\/\S+\.\S+/i.test(raw)) {
      previewLoading = false
      return
    }
    previewLoading = true
    let cancelled = false
    const t = setTimeout(async () => {
      try {
        const info = await GetPreview(raw)
        if (!cancelled) preview = info.kind === 'none' ? null : info
      } catch {
        if (!cancelled) preview = null
      } finally {
        if (!cancelled) previewLoading = false
      }
    }, 600)
    return () => {
      cancelled = true
      clearTimeout(t)
    }
  })
  // Fetch the video's real available resolutions whenever the URL settles.
  // Same debounce-and-cancel pattern as the preview effect above: the cleanup
  // runs before each re-run, cancelling the pending timer while the user is
  // still typing and flagging in-flight responses as stale.
  $effect(() => {
    const raw = url.trim()
    availableFormats = null
    audioExt = ''
    if (!installed || !/^https?:\/\/\S+\.\S+/i.test(raw)) {
      formatsLoading = false
      return
    }
    formatsLoading = true
    let cancelled = false
    const t = setTimeout(async () => {
      try {
        const list = await ListFormats(raw)
        if (!cancelled) {
          availableFormats = list.videos ?? []
          audioExt = list.audioExt ?? ''
          // If the user pre-picked a height this video doesn't offer, snap to
          // the closest available one so the dropdown never shows a blank.
          const heights = (list.videos ?? []).map((v) => v.height)
          const h = Number(quality)
          if (heights.length && !Number.isNaN(h) && quality !== 'best' && quality !== 'audio' && !heights.includes(h)) {
            const closest = heights.reduce((a, b) => (Math.abs(b - h) < Math.abs(a - h) ? b : a))
            quality = String(closest)
          }
        }
      } catch {
        if (!cancelled) availableFormats = [] // fall back to presets
      } finally {
        if (!cancelled) formatsLoading = false
      }
    }, 600)
    return () => {
      cancelled = true
      clearTimeout(t)
    }
  })

  let clipStartValid = $derived(clipStart.trim() === '' || timeRe.test(clipStart.trim()))
  let clipEndValid = $derived(clipEnd.trim() === '' || timeRe.test(clipEnd.trim()))
  // Both fields empty = no clipping, which is always fine.
  let clipOk = $derived(clipStartValid && clipEndValid)

  // onMount runs once after the component first renders (like useEffect with []).
  // Returning a function from onMount registers cleanup that runs on unmount —
  // here we use it to unsubscribe from the event listeners. EventsOn returns
  // its own unsubscribe function, which we call in that cleanup.
  onMount(() => {
    // Check install state. Wrapped in an inner async fn because onMount's own
    // return value is reserved for the cleanup function above.
    ;(async () => {
      await refreshDeps() // also sets ffmpegOk from the deps status
      previewPort = await PreviewPort()
    })()

    const offProgress = EventsOn('download:progress', (p) => {
      percent = p.percent
      speed = p.speed
      eta = p.eta
    })
    const offLog = EventsOn('download:log', (line: string) => {
      step = line
    })

    return () => {
      offProgress()
      offLog()
    }
  })

  async function refreshDeps() {
    deps = await GetDepsStatus()
    ffmpegOk = deps.ffmpeg
    if (deps.python && deps.ytdlp && deps.extras && !version) {
      try {
        version = await YtDlpVersion()
      } catch {
        version = ''
      }
    }
  }

  async function installDep(target: DepKey) {
    confirmTarget = null
    busy = true
    percent = 0
    step = ''
    status = `Downloading ${depInfo[target].name}…`
    try {
      if (target === 'python') await InstallPython()
      else if (target === 'ytdlp') await InstallYtDlpZip()
      else if (target === 'ffmpeg') await InstallFfmpeg()
      else await InstallExtras()
      status = `${depInfo[target].name}: installed.`
    } catch (err) {
      status = `Install failed: ${err}`
    } finally {
      busy = false
      percent = 0
      step = ''
      await refreshDeps()
    }
  }

  async function download() {
    busy = true
    percent = 0
    speed = ''
    eta = ''
    step = ''
    status = ''
    try {
      // Section times only apply when ffmpeg is available (inputs are
      // disabled otherwise, but guard anyway).
      const start = ffmpegOk ? clipStart.trim() : ''
      const end = ffmpegOk ? clipEnd.trim() : ''
      await DownloadVideo(url, start, end, quality)
      status = 'Done! Saved to your Downloads folder.'
    } catch (err) {
      status = `${err}`
    } finally {
      busy = false
    }
  }
</script>

<main>
  <h1>yt-dlp GUI</h1>
  <p class="tagline">Enter any video url to download it</p>

  {#if !setupDone}
    <div class="setup">
      <p class="setup-title">A few components are needed before downloading videos:</p>
      {#each depKeys as dep (dep)}
        <div class="dep-row">
          <div class="dep-text">
            <span class="dep-name">
              {depInfo[dep].name}
              <span class="dep-size">({depInfo[dep].size})</span>
              {#if depInfo[dep].optional}<span class="dep-optional">optional</span>{/if}
            </span>
            <span class="dep-desc">{depInfo[dep].short}</span>
          </div>
          {#if deps[dep]}
            <span class="dep-done">✓ Installed</span>
          {:else}
            <div class="dep-actions">
              <button
                onclick={() => (confirmTarget = dep)}
                disabled={busy || (dep === 'extras' && !deps.python)}
                title={dep === 'extras' && !deps.python ? 'Requires the Python runtime' : undefined}
              >
                Download…
              </button>
              {#if dep === 'ffmpeg'}
                <button onclick={skipFfmpeg} disabled={busy}>Skip</button>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <p class="version">yt-dlp {version} ready</p>
  {/if}

  {#if confirmTarget}
    {@const info = depInfo[confirmTarget]}
    <!-- Confirmation modal: nothing is downloaded until the user approves. -->
    <div
      class="modal-backdrop"
      role="presentation"
      onclick={() => (confirmTarget = null)}
    >
      <div
        class="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        onclick={(e) => e.stopPropagation()}
      >
        <h2 id="modal-title">Download {info.name}?</h2>
        <p class="modal-blurb">{info.blurb}</p>
        <p class="modal-source">
          Download size: {info.size}<br />
          Source: <strong>{info.source}</strong>
        </p>
        <div class="modal-actions">
          <button onclick={() => (confirmTarget = null)}>Cancel</button>
          <button onclick={() => installDep(confirmTarget!)}>Download</button>
        </div>
      </div>
    </div>
  {/if}

  <div class="row">
    <input
      class="input url-input"
      type="text"
      placeholder="Paste a video URL…"
      bind:value={url}
      autocomplete="off"
    />
    <button
      class="download-btn"
      onclick={download}
      disabled={busy || !installed || url.trim() === '' || !clipOk}
    >
      Download
    </button>
  </div>

  {#if previewUrl}
    <!-- YouTube fast path. {#key} destroys and recreates the iframe when the
         URL changes, so pasting a new video loads fresh instead of reusing
         the old player. -->
    {#key previewUrl}
      <div class="preview">
        <iframe
          src={previewUrl}
          title="Video preview"
          allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowfullscreen
        ></iframe>
      </div>
    {/key}
  {:else if preview?.kind === 'embed'}
    <!-- Other sites: official player embed discovered via oEmbed. -->
    {#key preview.url}
      <div class="preview">
        <iframe
          src={preview.url}
          title={preview.title || 'Video preview'}
          allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowfullscreen
        ></iframe>
      </div>
    {/key}
  {:else if preview?.kind === 'thumbnail'}
    <!-- No embeddable player available — show the thumbnail instead. -->
    <div class="preview thumb">
      <img src={preview.thumbnail} alt={preview.title || 'Video thumbnail'} />
      {#if preview.title || preview.duration}
        <div class="thumb-caption">
          <span class="thumb-title">{preview.title}</span>
          {#if preview.duration}<span class="thumb-duration">{preview.duration}</span>{/if}
        </div>
      {/if}
    </div>
  {:else if previewLoading}
    <p class="preview-loading">Looking up preview…</p>
  {/if}

  <div class="quality">
    <label class="quality-label">
      <span>Quality</span>
      <!-- bind:value on a <select> works like bind:value on an <input>: pick
           an option and `quality` updates; set `quality` in code and the
           dropdown follows. Option values are strings, hence String(h). -->
      <select bind:value={quality} disabled={busy}>
        <option value="best">Best available</option>
        {#each formatOptions as f (f.height)}
          <option value={String(f.height)}>{qualityLabel(f)}</option>
        {/each}
        <option value="audio">Audio only{audioExt ? ` · ${audioExt}` : ''}</option>
      </select>
    </label>
    {#if formatsLoading}
      <span class="quality-note">checking available qualities…</span>
    {:else if availableFormats && availableFormats.length > 0}
      <span class="quality-note ok">showing this video’s qualities</span>
    {/if}
  </div>

  <!-- Always-visible section card. When ffmpeg is missing the whole card is
       dimmed and acts as one big button that re-opens the ffmpeg install
       prompt — the disabled inputs let the click fall through to the card. -->
  <div
    class="clip"
    class:disabled={!ffmpegOk}
    role="presentation"
    onclick={() => {
      if (!ffmpegOk && !busy) confirmTarget = 'ffmpeg'
    }}
  >
    <div class="clip-head">
      <span class="clip-title">Clip</span>
      {#if !ffmpegOk}
        <span class="clip-need">requires ffmpeg — click to install</span>
      {/if}
    </div>

    <div class="clip-row">
      <label class="clip-field">
        <span>Start</span>
        <input
          class="input time-input"
          class:invalid={!clipStartValid}
          type="text"
          placeholder="0:30"
          bind:value={clipStart}
          disabled={busy || !ffmpegOk}
          autocomplete="off"
        />
      </label>
      <span class="clip-dash">–</span>
      <label class="clip-field">
        <span>Stop</span>
        <input
          class="input time-input"
          class:invalid={!clipEndValid}
          type="text"
          placeholder="1:45"
          bind:value={clipEnd}
          disabled={busy || !ffmpegOk}
          autocomplete="off"
        />
      </label>
    </div>
    <p class="clip-hint">
      Leave both blank to download the whole video. Use seconds (90), MM:SS
      (1:30), or HH:MM:SS — leave one side blank to clip from the beginning or
      to the end.
    </p>
  </div>

  {#if busy || percent > 0}
    <div class="progress" role="progressbar" aria-valuenow={percent} aria-valuemin="0" aria-valuemax="100">
      <div class="progress-fill" style="width: {percent}%"></div>
    </div>
    <p class="meta">
      {percent.toFixed(1)}%{speed ? ` · ${speed}` : ''}{eta ? ` · ETA ${eta}` : ''}
    </p>
    {#if step}
      <p class="step">{step}</p>
    {/if}
  {/if}

  {#if status}
    <p class="status">{status}</p>
  {/if}
</main>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape') confirmTarget = null
  }}
/>

<style>
  main {
    max-width: 640px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    text-align: center;
  }

  h1 {
    font-weight: 700;
    letter-spacing: -0.02em;
    margin-bottom: 0.25rem;
  }

  .tagline {
    opacity: 0.6;
    font-size: 0.95rem;
    margin-bottom: 1.5rem;
  }

  .setup {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.12);
    padding: 0.75rem 1rem 0.5rem;
    border-radius: 8px;
    margin-bottom: 1.25rem;
    text-align: left;
  }

  .setup-title {
    font-size: 0.9rem;
    margin: 0.25rem 0 0.5rem;
  }

  .dep-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.5rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
  }

  .dep-text {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .dep-name {
    font-size: 0.9rem;
    font-weight: 600;
  }

  .dep-size {
    font-weight: 400;
    opacity: 0.55;
  }

  .dep-desc {
    font-size: 0.8rem;
    opacity: 0.6;
  }

  .dep-done {
    font-size: 0.85rem;
    color: #5dc06b;
    white-space: nowrap;
  }

  .dep-actions {
    display: flex;
    gap: 0.4rem;
    white-space: nowrap;
  }

  .dep-optional {
    font-size: 0.65rem;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
    border: 1px solid rgba(255, 255, 255, 0.3);
    border-radius: 999px;
    padding: 0.1rem 0.45rem;
    vertical-align: middle;
  }

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;
  }

  .modal {
    width: min(440px, calc(100vw - 3rem));
    background: #28282a;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    text-align: left;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  .modal h2 {
    margin: 0 0 0.75rem;
    font-size: 1.1rem;
  }

  .modal-blurb {
    font-size: 0.88rem;
    line-height: 1.45;
    opacity: 0.85;
    margin: 0 0 0.75rem;
  }

  .modal-source {
    font-size: 0.85rem;
    opacity: 0.8;
    margin: 0 0 1rem;
    line-height: 1.5;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }

  .version {
    opacity: 0.6;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  /* Buttons are intentionally left unstyled (no background/border/appearance
     overrides). The OS webview — WKWebView on macOS, WebView2 on Windows —
     then renders its platform's default control, and `color-scheme: dark`
     in style.css makes it draw the dark variant. Styling any of those
     properties would replace the native look with a generic CSS box. */
  button {
    font-size: 0.95rem;
  }

  /* Native controls scale with their font size, so bumping the font is the
     one way to get a BIGGER button without breaking the platform-native
     rendering (explicit width/height/padding would). */
  .download-btn {
    font-size: 1.25rem;
  }

  /* Text inputs get minimal styling that doesn't break the native field
     rendering on either platform. */
  .input {
    font-size: 1rem;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background-color: rgba(255, 255, 255, 0.08);
    color: #eee;
    outline: none;
  }

  .input:focus {
    border-color: #4f9dff;
  }

  .input.invalid {
    border-color: #e5534b;
  }

  .url-input {
    flex: 1;
  }

  .preview {
    margin-top: 1rem;
    aspect-ratio: 16 / 9;
    border-radius: 10px;
    overflow: hidden;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.1);
  }

  .preview iframe {
    width: 100%;
    height: 100%;
    border: 0;
    display: block;
  }

  .preview.thumb {
    position: relative;
  }

  .preview.thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .thumb-caption {
    position: absolute;
    inset: auto 0 0 0;
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 1.25rem 0.75rem 0.5rem;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.75), transparent);
    text-align: left;
  }

  .thumb-title {
    font-size: 0.85rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .thumb-duration {
    font-size: 0.8rem;
    opacity: 0.8;
    font-variant-numeric: tabular-nums;
  }

  .preview-loading {
    margin-top: 1rem;
    font-size: 0.85rem;
    opacity: 0.5;
  }

  .quality {
    margin-top: 1rem;
    text-align: left;
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .quality-label {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
  }

  /* Like buttons, <select> is left visually unstyled so the OS webview renders
     its native dropdown control (see the button comment above). */
  .quality-label select {
    font-size: 0.9rem;
  }

  .quality-note {
    font-size: 0.78rem;
    opacity: 0.5;
  }

  .quality-note.ok {
    color: #5dc06b;
    opacity: 0.8;
  }

  /* The section card matches the setup panel's visual language (same subtle
     surface + border) so the page reads as a column of consistent cards. */
  .clip {
    margin-top: 1rem;
    text-align: left;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 0.75rem 1rem;
    transition: opacity 0.15s ease;
  }

  .clip.disabled {
    opacity: 0.55;
    cursor: pointer;
  }

  .clip.disabled:hover {
    opacity: 0.75;
  }

  .clip-head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .clip-title {
    font-size: 0.9rem;
    font-weight: 600;
  }

  .clip-need {
    font-size: 0.78rem;
    color: #f0b429;
    margin-left: auto;
  }

  .clip-row {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .clip-field {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
    font-size: 0.8rem;
    opacity: 0.9;
  }

  .time-input {
    width: 90px;
    font-variant-numeric: tabular-nums;
  }

  .clip-dash {
    padding-bottom: 6px;
    opacity: 0.5;
  }

  .clip-hint {
    margin-top: 0.5rem;
    margin-bottom: 0;
    font-size: 0.78rem;
    opacity: 0.5;
  }

  .progress {
    margin-top: 1.5rem;
    height: 10px;
    width: 100%;
    background: rgba(255, 255, 255, 0.12);
    border-radius: 999px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(to right, #4f9dff, #8ad);
    border-radius: 999px;
    transition: width 0.2s ease;
  }

  .meta {
    margin-top: 0.5rem;
    font-size: 0.85rem;
    opacity: 0.8;
    font-variant-numeric: tabular-nums;
  }

  .step {
    margin-top: 0.25rem;
    font-size: 0.8rem;
    opacity: 0.6;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .status {
    margin-top: 1.25rem;
    white-space: pre-wrap;
    text-align: left;
    font-size: 0.9rem;
    opacity: 0.85;
  }
</style>

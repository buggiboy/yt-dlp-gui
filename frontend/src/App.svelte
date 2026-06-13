<script lang="ts">
  import { onMount } from 'svelte'
  import {
    DownloadsFolder,
    GetPreview,
    ListFormats,
    PreviewPort,
  } from '../wailsjs/go/main/App.js'
  import { BrowserOpenURL, EventsOn } from '../wailsjs/runtime/runtime.js'
  import type { VideoFormat } from './lib/types'

  import { DepsState } from './lib/state/deps.svelte'
  import { DownloadState } from './lib/state/download.svelte'
  import { buildCommand } from './lib/command'
  import { compile } from './lib/filename/compile'
  import { preview as previewFilename } from './lib/filename/preview'

  import SetupPanel from './components/SetupPanel.svelte'
  import ConfirmModal from './components/ConfirmModal.svelte'
  import VideoPreview from './components/VideoPreview.svelte'
  import QualityPicker from './components/QualityPicker.svelte'
  import ClipSection from './components/ClipSection.svelte'
  import AdvancedOptions from './components/AdvancedOptions.svelte'
  import FilenameFormatModal from './components/FilenameFormatModal.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import ProgressBar from './components/ProgressBar.svelte'

  const dl = new DownloadState()
  const deps = new DepsState(() => dl.resetProgress())

  let previewPort = $state(0)
  let defaultFolder = $state('')
  let showFilenameModal = $state(false)
  let showSettings = $state(false)

  // Compiled filename format, shared by the command preview and the Advanced
  // row summary. Recomputes whenever the format changes.
  let compiledName = $derived(compile(dl.filenameFormat))
  let filenamePreview = $derived(previewFilename(dl.filenameFormat).name)

  // Extension the current quality choice will produce: audio downloads take the
  // chosen/known audio format, everything else lands as mp4 (matching the
  // command preview). Video merge ext can vary, but mp4 is the common result.
  let downloadExt = $derived(
    dl.quality === 'audio'
      ? (dl.audioFormat || dl.audioExtension || 'm4a')
      : 'mp4'
  )

  // Subtle "saves as" line shown under the URL. Uses the real video title once
  // a preview lookup resolves it, otherwise the format's sample stands in.
  let downloadFilename = $derived(
    previewFilename(dl.filenameFormat, {
      ext: downloadExt,
      meta: dl.preview?.title ? { title: dl.preview.title } : undefined,
    }).name
  )

  let anyBusy = $derived(deps.busy || dl.busy)
  let saveFolder = $derived(dl.folder || defaultFolder)
  let previewUrl = $derived(
    dl.videoId && previewPort
      ? `http://127.0.0.1:${previewPort}/player/${dl.videoId}`
      : null
  )
  // Show whichever status message is current — only one operation runs at a time.
  let status = $derived(deps.status || dl.status)

  // Live preview of the yt-dlp command the current options would run.
  let command = $derived(
    buildCommand({
      url: dl.url,
      quality: dl.quality,
      audioFormat: dl.audioFormat,
      clipStart: dl.clipStart,
      clipEnd: dl.clipEnd,
      folder: saveFolder,
      subtitles: dl.subtitles,
      subLangs: dl.subLangs,
      embedMeta: dl.embedMeta,
      sponsorBlock: dl.sponsorBlock,
      sponsorBlockCategories: dl.sponsorBlockCategories,
      rateLimit: dl.rateLimit,
      concurrentFragments: dl.concurrentFragments,
      extraArgs: dl.extraArgs,
      ffmpegAvailable: deps.ffmpegAvailable,
      filenameTemplate: compiledName.outtmpl,
      nameArgs: compiledName.nameArgs,
    })
  )

  const SUPPORTED_SITES_URL = "https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md"

  onMount(() => {
    // Wire up progress events. These fire during both downloads and installs,
    // so they update the shared progress state on DownloadState.
    const stopProgress = EventsOn('download:progress', (p) => {
      dl.percent = p.percent
      dl.speed = p.speed
      dl.eta = p.eta
    })
    const stopLog = EventsOn('download:log', (line: string) => {
      dl.step = line
    })

    ;(async () => {
      await deps.refresh()
      previewPort = await PreviewPort()
      try {
        defaultFolder = await DownloadsFolder()
      } catch {
        defaultFolder = ''
      }
      // Restore the saved filename format, if any.
      await dl.loadSettings()
    })()

    return () => {
      stopProgress()
      stopLog()
    }
  })

  // Debounced non-YouTube preview lookup. Cross-cutting: reads dl.url and
  // dl.videoId, checks previewPort (App state), writes back to dl.preview.
  // The cleanup cancels the pending timer on each re-run so rapid typing only
  // fires one request after the user pauses.
  $effect(() => {
    const raw = dl.url.trim()
    dl.preview = null
    if (dl.videoId || !previewPort || !/^https?:\/\/\S+\.\S+/i.test(raw)) {
      dl.loadingPreview = false
      return
    }
    dl.loadingPreview = true
    let cancelled = false
    const timer = setTimeout(async () => {
      try {
        const info = await GetPreview(raw)
        if (!cancelled) dl.preview = info.kind === 'none' ? null : info
      } catch {
        if (!cancelled) dl.preview = null
      } finally {
        if (!cancelled) dl.loadingPreview = false
      }
    }, 600)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  })

  // Debounced format list fetch — same debounce-and-cancel pattern as above.
  // Cross-cutting: reads deps.installed to gate the fetch.
  $effect(() => {
    const raw = dl.url.trim()
    dl.availableFormats = null
    dl.audioExtension = ''
    if (!deps.installed || !/^https?:\/\/\S+\.\S+/i.test(raw)) {
      dl.loadingFormats = false
      return
    }
    dl.loadingFormats = true
    let cancelled = false
    const timer = setTimeout(async () => {
      try {
        const list = await ListFormats(raw)
        if (!cancelled) {
          dl.availableFormats = list.videos ?? []
          dl.audioExtension = list.audioExt ?? ''
          // Snap quality to the closest available height if the current pick
          // isn't offered by this video.
          const heights = (list.videos ?? []).map((v: VideoFormat) => v.height)
          const selectedHeight = Number(dl.quality)
          if (
            heights.length &&
            !Number.isNaN(selectedHeight) &&
            dl.quality !== 'best' &&
            dl.quality !== 'audio' &&
            !heights.includes(selectedHeight)
          ) {
            const closest = heights.reduce((a: number, b: number) =>
              Math.abs(b - selectedHeight) < Math.abs(a - selectedHeight) ? b : a
            )
            dl.quality = String(closest)
          }
        }
      } catch {
        if (!cancelled) dl.availableFormats = []
      } finally {
        if (!cancelled) dl.loadingFormats = false
      }
    }, 600)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  })
</script>

<main>
  <button
    class="settings-button"
    onclick={() => (showSettings = true)}
    title="Settings"
    aria-label="Open settings"
  >
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  </button>

  <h1>yt-dlp GUI</h1>
  <p class="tagline">Enter any video url to download it. See a list of supported sites <button class="link-button" title={SUPPORTED_SITES_URL} onclick={() => BrowserOpenURL(SUPPORTED_SITES_URL)}>here</button></p> 
  

  {#if deps.confirmTarget}
    <ConfirmModal
      target={deps.confirmTarget}
      onConfirm={() => deps.install(deps.confirmTarget!)}
      onCancel={() => (deps.confirmTarget = null)}
    />
  {/if}

  <div class="url-row">
    <input
      class="url-input"
      type="text"
      placeholder="Paste a video URL…"
      bind:value={dl.url}
      autocomplete="off"
    />
    <button
      class="download-button primary"
      onclick={() => dl.download({ ffmpegAvailable: deps.ffmpegAvailable, saveFolder })}
      disabled={anyBusy || !deps.installed || dl.url.trim() === '' || !dl.clipValid}
    >
      Download
    </button>
  </div>

  {#if dl.url.trim()}
    <p class="filename-preview" title="The name this download will be saved as">
      <span class="fp-label">Saves as</span>
      <span class="fp-name">{downloadFilename}</span>
    </p>
  {/if}

  <VideoPreview {previewUrl} preview={dl.preview} loading={dl.loadingPreview} />

  <div class="format-row">
    <QualityPicker
      bind:quality={dl.quality}
      bind:audioFormat={dl.audioFormat}
      formatOptions={dl.formatOptions}
      audioExtension={dl.audioExtension}
      loadingFormats={dl.loadingFormats}
      availableFormats={dl.availableFormats}
      ffmpegAvailable={deps.ffmpegAvailable}
      busy={anyBusy}
    />

    <ClipSection
      bind:clipStart={dl.clipStart}
      bind:clipEnd={dl.clipEnd}
      ffmpegAvailable={deps.ffmpegAvailable}
      busy={anyBusy}
      onRequestFfmpeg={() => (deps.confirmTarget = 'ffmpeg')}
    />
  </div>

  <AdvancedOptions
    bind:folder={dl.folder}
    defaultFolder={defaultFolder}
    bind:subtitles={dl.subtitles}
    bind:subLangs={dl.subLangs}
    bind:embedMeta={dl.embedMeta}
    bind:sponsorBlock={dl.sponsorBlock}
    bind:sponsorBlockCategories={dl.sponsorBlockCategories}
    bind:extraArgs={dl.extraArgs}
    filenamePreview={filenamePreview}
    ffmpegAvailable={deps.ffmpegAvailable}
    busy={anyBusy}
    onChooseFolder={() => dl.chooseFolder()}
    onEditFilename={() => (showFilenameModal = true)}
  />

  {#if showFilenameModal}
    <FilenameFormatModal
      format={dl.filenameFormat}
      onApply={(f) => { dl.setFilenameFormat(f); showFilenameModal = false }}
      onCancel={() => (showFilenameModal = false)}
    />
  {/if}

  {#if showSettings}
    <SettingsModal
      rateLimit={dl.rateLimit}
      onRateLimitChange={(v) => dl.setRateLimit(v)}
      concurrentFragments={dl.concurrentFragments}
      onConcurrentFragmentsChange={(v) => dl.setConcurrentFragments(v)}
      onClose={() => (showSettings = false)}
      onOpenDepsFolder={() => dl.openDepsFolder()}
    />
  {/if}

  <ProgressBar busy={anyBusy} percent={dl.percent} speed={dl.speed} eta={dl.eta} step={dl.step} />

  {#if status}
    <p class="status">{status}</p>
  {/if}

  {#if command}
    <p class="command" title="The yt-dlp command these options will run">{command}</p>
  {/if}

  {#if !deps.setupComplete}
    <SetupPanel
      deps={deps.deps}
      busy={anyBusy}
      onRequestInstall={(key) => (deps.confirmTarget = key)}
      onSkipFfmpeg={() => deps.skip()}
    />
  {:else}
    <p class="version">Using yt-dlp {deps.version}</p>
  {/if}
</main>

<style>
  main {
    position: relative;
    max-width: 640px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    text-align: center;
  }

  /* Gear button pinned to the top-right corner of the app. Icon-only, opts
     out of the global push-button look. */
  .settings-button {
    position: absolute;
    top: 1rem;
    right: 1rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 7px;
    background: none;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    transition: color 0.12s ease, transform 0.18s ease;
  }

  .settings-button:hover {
    background: none;
    color: var(--text);
    transform: rotate(45deg);
  }

  .settings-button svg {
    display: block;
  }

  h1 {
    font-size: 1.9rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    margin-bottom: 0.25rem;
  }

  .tagline {
    color: var(--text-dim);
    font-size: var(--fs-lg);
    margin-bottom: 1.5rem;
  }

  .version {
    color: var(--text-dim);
    font-size: var(--fs-base);
    margin-bottom: 1.25rem;
  }

  .url-row {
    display: flex;
    align-items: stretch;
    gap: 0.5rem;
  }

  /* Subtle echo of the resulting filename, sitting just under the URL field. */
  .filename-preview {
    display: flex;
    align-items: baseline;
    gap: 0.4rem;
    margin: 0.5rem 0 0;
    padding: 0 0.1rem;
    text-align: left;
    font-size: var(--fs-sm);
    min-width: 0;
  }

  .fp-label {
    flex-shrink: 0;
    color: var(--text-faint);
  }

  .fp-name {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--mono);
    color: var(--text-dim);
  }

  /* Quality selector and Clip controls share one row, wrapping as needed. */
  .format-row {
    margin-top: 1rem;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem 1.5rem;
  }

  /* Primary URL field — a touch larger than secondary inputs since it's the
     main entry point. Inherits the shared input look from style.css. */
  .url-input {
    flex: 1;
    font-size: var(--fs-lg);
    padding: 8px 12px;
  }

  .link-button {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
    font-size: inherit;
  }

  /* Primary action, sized to the URL field beside it. */
  .download-button {
    font-size: var(--fs-lg);
    font-weight: 600;
    padding: 8px 20px;
  }

  .status {
    margin-top: 1.25rem;
    white-space: pre-wrap;
    text-align: left;
    font-size: var(--fs-base);
    opacity: 0.85;
  }

  /* Subtle, monospace echo of the command the current options will run. Sits
     quietly at the bottom; selectable so it can be copied. */
  .command {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.75rem;
    line-height: 1.5;
    text-align: left;
    color: rgba(255, 255, 255, 0.35);
    word-break: break-all;
    white-space: pre-wrap;
    user-select: text;
  }
</style>

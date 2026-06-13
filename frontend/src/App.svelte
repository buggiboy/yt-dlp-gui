<script lang="ts">
  import { onMount } from 'svelte'
  import {
    DownloadsFolder,
    GetPreview,
    ListFormats,
    PreviewPort,
  } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import type { VideoFormat } from './lib/types'

  import { DepsState } from './lib/state/deps.svelte'
  import { DownloadState } from './lib/state/download.svelte'

  import SetupPanel from './components/SetupPanel.svelte'
  import ConfirmModal from './components/ConfirmModal.svelte'
  import VideoPreview from './components/VideoPreview.svelte'
  import QualityPicker from './components/QualityPicker.svelte'
  import ClipSection from './components/ClipSection.svelte'
  import AdvancedOptions from './components/AdvancedOptions.svelte'
  import ProgressBar from './components/ProgressBar.svelte'

  const dl = new DownloadState()
  const deps = new DepsState(() => dl.resetProgress())

  let previewPort = $state(0)
  let defaultFolder = $state('')

  let anyBusy = $derived(deps.busy || dl.busy)
  let saveFolder = $derived(dl.folder || defaultFolder)
  let previewUrl = $derived(
    dl.videoId && previewPort
      ? `http://127.0.0.1:${previewPort}/player/${dl.videoId}`
      : null
  )
  // Show whichever status message is current — only one operation runs at a time.
  let status = $derived(deps.status || dl.status)

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
  <h1>yt-dlp GUI</h1>
  <p class="tagline">Enter any video url to download it</p>

  {#if !deps.setupComplete}
    <SetupPanel
      deps={deps.deps}
      busy={anyBusy}
      onRequestInstall={(key) => (deps.confirmTarget = key)}
      onSkipFfmpeg={() => deps.skip()}
    />
  {:else}
    <p class="version">yt-dlp {deps.version} ready</p>
  {/if}

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
      class="download-button"
      onclick={() => dl.download({ ffmpegAvailable: deps.ffmpegAvailable, saveFolder })}
      disabled={anyBusy || !deps.installed || dl.url.trim() === '' || !dl.clipValid}
    >
      Download
    </button>
  </div>

  <VideoPreview {previewUrl} preview={dl.preview} loading={dl.loadingPreview} />

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

  <AdvancedOptions
    bind:folder={dl.folder}
    defaultFolder={defaultFolder}
    bind:subtitles={dl.subtitles}
    bind:subLangs={dl.subLangs}
    bind:embedMeta={dl.embedMeta}
    bind:sponsorBlock={dl.sponsorBlock}
    bind:sponsorBlockCategories={dl.sponsorBlockCategories}
    ffmpegAvailable={deps.ffmpegAvailable}
    busy={anyBusy}
    onChooseFolder={() => dl.chooseFolder()}
    onOpenDepsFolder={() => dl.openDepsFolder()}
  />

  <ProgressBar busy={anyBusy} percent={dl.percent} speed={dl.speed} eta={dl.eta} step={dl.step} />

  {#if status}
    <p class="status">{status}</p>
  {/if}
</main>

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

  .version {
    opacity: 0.6;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .url-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  /* Text inputs get minimal styling that avoids breaking native field
     rendering on either platform. */
  .url-input {
    flex: 1;
    font-size: 1rem;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background-color: rgba(255, 255, 255, 0.08);
    color: #eee;
    outline: none;
  }

  .url-input:focus {
    border-color: #4f9dff;
  }

  /* Buttons are intentionally left unstyled so the OS webview renders its
     platform's native control. Styling background/border/appearance would
     replace that with a generic CSS box. */
  button {
    font-size: 0.95rem;
  }

  .download-button {
    font-size: 1.25rem;
  }

  .status {
    margin-top: 1.25rem;
    white-space: pre-wrap;
    text-align: left;
    font-size: 0.9rem;
    opacity: 0.85;
  }
</style>

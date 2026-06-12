<script lang="ts">
  import { onMount } from 'svelte'
  import {
    DownloadVideo,
    InstallYtDlp,
    YtDlpInstalled,
    YtDlpVersion,
  } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'

  // --- Svelte 5 runes ---
  // $state() makes a variable reactive. Reassigning it re-renders the UI,
  // similar to a useState value in React — but you mutate it directly,
  // there's no setter function.
  let url = $state('')
  let installed = $state(false)
  let version = $state('')
  let status = $state('')
  let busy = $state(false)

  // Live download progress, driven by Wails events emitted from Go.
  let percent = $state(0)
  let speed = $state('')
  let eta = $state('')
  let step = $state('')

  // onMount runs once after the component first renders (like useEffect with []).
  // Returning a function from onMount registers cleanup that runs on unmount —
  // here we use it to unsubscribe from the event listeners. EventsOn returns
  // its own unsubscribe function, which we call in that cleanup.
  onMount(() => {
    // Check install state. Wrapped in an inner async fn because onMount's own
    // return value is reserved for the cleanup function above.
    ;(async () => {
      installed = await YtDlpInstalled()
      if (installed) version = await YtDlpVersion()
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

  async function install() {
    busy = true
    status = 'Downloading the latest yt-dlp…'
    try {
      version = await InstallYtDlp()
      installed = true
      status = `Installed yt-dlp ${version}`
    } catch (err) {
      status = `Install failed: ${err}`
    } finally {
      busy = false
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
      await DownloadVideo(url)
      status = 'Done! Saved to your Downloads folder.'
    } catch (err) {
      status = `${err}`
    } finally {
      busy = false
    }
  }
</script>

<main>
  <h1>yt-dlp</h1>

  {#if !installed}
    <div class="banner">
      <span>yt-dlp isn't installed yet.</span>
      <button class="btn" onclick={install} disabled={busy}>
        Download latest yt-dlp
      </button>
    </div>
  {:else}
    <p class="version">yt-dlp {version} ready</p>
  {/if}

  <div class="row">
    <input
      class="input"
      type="text"
      placeholder="Paste a YouTube URL…"
      bind:value={url}
      autocomplete="off"
    />
    <button
      class="btn primary"
      onclick={download}
      disabled={busy || !installed || url.trim() === ''}
    >
      Download
    </button>
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
    margin-bottom: 1.5rem;
  }

  .banner {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    background: rgba(255, 255, 255, 0.08);
    padding: 0.75rem 1rem;
    border-radius: 8px;
    margin-bottom: 1.25rem;
  }

  .version {
    opacity: 0.7;
    font-size: 0.9rem;
    margin-bottom: 1.25rem;
  }

  .row {
    display: flex;
    gap: 0.5rem;
  }

  .input {
    flex: 1;
    height: 40px;
    padding: 0 12px;
    border: none;
    border-radius: 6px;
    outline: none;
    font-size: 1rem;
    background-color: rgba(240, 240, 240, 1);
  }

  .btn {
    height: 40px;
    padding: 0 16px;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.95rem;
    background-image: linear-gradient(to top, #cfd9df 0%, #e2ebf0 100%);
    color: #1b2636;
  }

  .btn:hover:not(:disabled) {
    background-image: linear-gradient(to top, #e2ebf0 0%, #ffffff 100%);
  }

  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn.primary {
    font-weight: 600;
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

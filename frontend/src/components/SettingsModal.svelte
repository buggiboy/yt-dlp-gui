<script lang="ts">
  import InfoTip from './InfoTip.svelte'

  let {
    rateLimit,
    onRateLimitChange,
    concurrentFragments,
    onConcurrentFragmentsChange,
    onClose,
    onOpenDepsFolder,
  }: {
    rateLimit: string
    onRateLimitChange: (value: string) => void
    concurrentFragments: number
    onConcurrentFragmentsChange: (value: number) => void
    onClose: () => void
    onOpenDepsFolder: () => void
  } = $props()

  // Offered presets for parallel fragment downloads. 0 = off (yt-dlp's default
  // of 1, sequential). The dropdown is intentionally a short, legible set rather
  // than a freeform number.
  const FRAGMENT_OPTIONS = [
    { value: 0, label: 'Off' },
    { value: 4, label: '4' },
    { value: 8, label: '8' },
    { value: 16, label: '16' },
  ] as const

  // Sidebar tabs. Add new panes here as settings grow.
  const TABS = [
    { id: 'downloads', label: 'Downloads' },
    { id: 'deps', label: 'Dependencies' },
  ] as const

  type TabId = (typeof TABS)[number]['id']

  let activeTab = $state<TabId>('downloads')

  // --- Speed limit ---
  // The limit is always expressed in MB/s. It's stored in yt-dlp's number+unit
  // form with a fixed "M" suffix (e.g. "5M"); "" means unlimited. The field only
  // accepts the numeric MB/s value — the unit is fixed in the UI.

  /** Extracts the numeric MB/s portion of a stored rate ("5M" → "5"). Returns ""
   *  when the value is empty, so the field shows its placeholder. */
  function rateToField(value: string): string {
    const m = value.trim().match(/^([\d.]+)/)
    return m ? m[1] : ''
  }

  let speedField = $derived(rateToField(rateLimit))

  /** Serializes the field's raw text back to a yt-dlp rate. Blank or non-positive
   *  input means no limit. The unit is always MB ("M"). */
  function setSpeed(raw: string): void {
    const trimmed = raw.trim()
    const n = Number(trimmed)
    if (trimmed === '' || !Number.isFinite(n) || n <= 0) {
      onRateLimitChange('')
      return
    }
    onRateLimitChange(`${n}M`)
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape') onClose() }} />

<!-- Backdrop dismissal is a mouse convenience; Escape (handled above) is the
     keyboard-accessible way to close, so the static-element interaction here is
     intentional. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="backdrop" role="presentation" onclick={onClose}>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-labelledby="settings-title"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
  >
    <!-- Sidebar -->
    <nav class="sidebar" aria-label="Settings sections">
      <h2 id="settings-title" class="sidebar-title">Settings</h2>
      {#each TABS as tab (tab.id)}
        <button
          class="tab"
          class:active={activeTab === tab.id}
          aria-current={activeTab === tab.id ? 'page' : undefined}
          onclick={() => (activeTab = tab.id)}
        >
          {tab.label}
        </button>
      {/each}
    </nav>

    <!-- Content pane -->
    <section class="content">
      <button class="close" onclick={onClose} aria-label="Close settings" title="Close">×</button>

      {#if activeTab === 'downloads'}
        <h3 class="pane-title">Downloads</h3>
        <p class="pane-desc">
          Control how downloads behave — including how much of your connection
          they're allowed to use.
        </p>

        <div class="setting">
          <label class="field-label" for="speed-limit">Speed limit</label>
          <div class="field-row">
            <input
              id="speed-limit"
              class="speed-input"
              type="number"
              inputmode="decimal"
              min="0"
              step="0.5"
              placeholder="No limit"
              value={speedField}
              oninput={(e) => setSpeed(e.currentTarget.value)}
              aria-label="Maximum download speed in megabytes per second"
            />
            <span class="unit">MB/s</span>
          </div>
          <p class="setting-hint">
            Caps the download rate so it doesn't saturate your connection.
            Leave blank for full speed.
          </p>
        </div>

        <div class="setting">
          <div class="field-head">
            <label class="field-label" for="parallel-fragments">Parallel fragments</label>
            <InfoTip label="About parallel fragments" align="left" width={280}>
              Many sites (YouTube, Twitch, and other DASH/HLS streams) deliver a
              video as lots of small fragments. Normally yt-dlp fetches them one at
              a time; this downloads several at once, hiding network latency and
              often speeding things up substantially.
              <br /><br />
              It has no effect on plain single-file downloads. Higher values give
              diminishing returns past 4–8 and use more bandwidth and CPU — some
              sites may also rate-limit aggressive parallelism.
            </InfoTip>
          </div>
          <div class="field-row">
            <select
              id="parallel-fragments"
              class="fragment-select"
              onchange={(e) => onConcurrentFragmentsChange(Number(e.currentTarget.value))}
            >
              {#each FRAGMENT_OPTIONS as opt (opt.value)}
                <option value={opt.value} selected={opt.value === concurrentFragments}>
                  {opt.label}
                </option>
              {/each}
            </select>
          </div>
          <p class="setting-hint">
            Downloads video fragments in parallel — a big speedup on many sites.
            Leave Off for normal sequential downloads.
          </p>
        </div>
      {:else if activeTab === 'deps'}
        <h3 class="pane-title">Dependencies</h3>
        <p class="pane-desc">
          Python, yt-dlp &amp; ffmpeg are downloaded and stored locally by the
          app.
        </p>
        <div class="row">
          <span class="row-label">Dependencies folder</span>
          <span class="path">Python, yt-dlp &amp; ffmpeg</span>
          <button
            onclick={onOpenDepsFolder}
            title="Open the folder where the app's dependencies are stored"
          >
            Open
          </button>
        </div>
      {/if}
    </section>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;
  }

  .modal {
    display: flex;
    width: min(640px, calc(100vw - 3rem));
    height: min(420px, calc(100vh - 4rem));
    background: #28282a;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
    text-align: left;
  }

  /* Sidebar — tab list down the left edge. */
  .sidebar {
    flex-shrink: 0;
    width: 180px;
    background: rgba(0, 0, 0, 0.18);
    border-right: 1px solid rgba(255, 255, 255, 0.08);
    padding: 1rem 0.6rem;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .sidebar-title {
    margin: 0 0 0.75rem;
    padding: 0 0.5rem;
    font-size: 1.05rem;
    font-weight: 700;
  }

  /* Text-style tab buttons — opt out of the global push-button look. */
  .tab {
    background: none;
    border: none;
    border-radius: var(--radius-control);
    padding: 0.5rem 0.6rem;
    text-align: left;
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-dim);
    cursor: pointer;
    transition: background 0.12s ease, color 0.12s ease;
  }

  .tab:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.06);
    border-color: transparent;
    color: var(--text);
  }

  .tab.active {
    background: rgba(255, 255, 255, 0.1);
    color: var(--text);
  }

  /* Content pane. */
  .content {
    position: relative;
    flex: 1;
    min-width: 0;
    padding: 1.25rem 1.5rem;
    overflow-y: auto;
  }

  .close {
    position: absolute;
    top: 0.75rem;
    right: 0.9rem;
    background: none;
    border: none;
    padding: 0;
    width: 1.6rem;
    height: 1.6rem;
    font-size: 1.4rem;
    line-height: 1;
    color: var(--text-dim);
    cursor: pointer;
  }

  .close:hover {
    background: none;
    color: var(--text);
  }

  .pane-title {
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
  }

  .pane-desc {
    margin: 0 0 1rem;
    font-size: var(--fs-base);
    line-height: 1.5;
    color: var(--text-dim);
  }

  /* Download settings. */
  .setting {
    padding: 0.4rem 0 0;
  }

  .setting + .setting {
    margin-top: 1.1rem;
    padding-top: 1.1rem;
    border-top: 1px solid rgba(255, 255, 255, 0.07);
  }

  .field-head {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.45rem;
  }

  .field-label {
    display: block;
    margin-bottom: 0.45rem;
    font-size: var(--fs-base);
    font-weight: 500;
  }

  /* Inside a field-head the label's own margin is replaced by the row gap. */
  .field-head .field-label {
    margin-bottom: 0;
  }

  .fragment-select {
    width: 6rem;
  }

  .field-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .speed-input {
    width: 6rem;
    flex-shrink: 0;
  }

  .unit {
    font-size: var(--fs-base);
    color: var(--text-dim);
  }

  .setting-hint {
    margin: 0.55rem 0 0;
    font-size: var(--fs-sm);
    line-height: 1.45;
    color: var(--text-dim);
  }

  /* Setting row — mirrors the layout used in Advanced options. */
  .row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.07);
    font-size: var(--fs-base);
  }

  .row-label {
    white-space: nowrap;
  }

  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: var(--fs-sm);
    color: var(--text-dim);
  }
</style>

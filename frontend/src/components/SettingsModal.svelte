<script lang="ts">
  import { untrack } from 'svelte'
  import InfoTip from './InfoTip.svelte'

  let {
    rateLimit,
    onRateLimitChange,
    concurrentFragments,
    onConcurrentFragmentsChange,
    ytDlpVersion,
    checking,
    updating,
    updateStatus,
    updateAvailable,
    latestVersion,
    changelogUrl,
    busy,
    onCheckForUpdate,
    onUpdateYtDlp,
    onOpenChangelog,
    onClose,
    onOpenDepsFolder,
    onResetSettings,
  }: {
    rateLimit: string
    onRateLimitChange: (value: string) => void
    concurrentFragments: number
    onConcurrentFragmentsChange: (value: number) => void
    ytDlpVersion: string
    checking: boolean
    updating: boolean
    updateStatus: string
    /** A check found a newer release that the user hasn't taken yet. */
    updateAvailable: boolean
    latestVersion: string
    /** Release notes (or a compare view) for everything since the installed
     *  version. Empty when there's nothing new to read about. */
    changelogUrl: string
    /** Another operation (a download or install) is running. Replacing the
     *  zipapp while it's being executed fails outright on Windows, so the
     *  update has to wait — checking is fine, it only talks to GitHub. */
    busy: boolean
    onCheckForUpdate: () => void
    onUpdateYtDlp: () => void
    onOpenChangelog: () => void
    onClose: () => void
    onOpenDepsFolder: () => void
    onResetSettings: () => void
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
    { id: 'general', label: 'General' },
  ] as const

  type TabId = (typeof TABS)[number]['id']

  let activeTab = $state<TabId>('downloads')

  // --- Reset ---
  // Two-step rather than a confirmation dialog: a modal on top of a modal is
  // worse than a button that asks once. Reverts on its own so a half-pressed
  // reset doesn't stay armed.
  let resetArmed = $state(false)
  let resetDone = $state(false)
  let disarmTimer: ReturnType<typeof setTimeout> | null = null

  function armReset(): void {
    resetArmed = true
    resetDone = false
    if (disarmTimer !== null) clearTimeout(disarmTimer)
    disarmTimer = setTimeout(() => (resetArmed = false), 5000)
  }

  function confirmReset(): void {
    if (disarmTimer !== null) clearTimeout(disarmTimer)
    disarmTimer = null
    resetArmed = false
    resetDone = true
    onResetSettings()
  }

  // --- Speed limit ---
  // Stored in yt-dlp's number+unit form ("500K", "5M"); "" means unlimited.
  // The UI splits that into a whole-number amount and a unit dropdown, which
  // avoids decimals entirely — "500 KB/s" says what "0.5 MB/s" would, without
  // asking a number field to cope with a half-typed "0.".

  type RateUnit = 'K' | 'M'

  const RATE_UNITS: { value: RateUnit; label: string }[] = [
    { value: 'K', label: 'KB/s' },
    { value: 'M', label: 'MB/s' },
  ]

  /** Splits a stored rate into its parts. Unknown or empty values fall back to
   *  "no amount, MB/s", which shows the placeholder. */
  function parseRate(value: string): { amount: number | null; unit: RateUnit } {
    const m = value.trim().match(/^(\d+)\s*([KM])?/i)
    if (!m) return { amount: null, unit: 'M' }
    return {
      amount: Number(m[1]),
      unit: (m[2]?.toUpperCase() as RateUnit) ?? 'M',
    }
  }

  // Local state, seeded once from the incoming value. The modal is created
  // fresh each time it opens, so this always starts from the saved setting —
  // and because the field isn't derived from the prop, a value round-tripping
  // through the parent can't overwrite what's being typed. untrack makes that
  // read-once intent explicit (and keeps the compiler from warning about it).
  const initialRate = untrack(() => parseRate(rateLimit))
  let amount = $state<number | null>(initialRate.amount)
  let unit = $state<RateUnit>(initialRate.unit)

  /** Pushes the current amount + unit up as a yt-dlp rate. A blank, zero, or
   *  negative amount means no limit. */
  function commitRate(): void {
    if (amount === null || !Number.isFinite(amount) || amount <= 0) {
      onRateLimitChange('')
      return
    }
    onRateLimitChange(`${Math.floor(amount)}${unit}`)
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape') onClose() }} />

<!-- Backdrop dismissal is a mouse convenience; Escape (handled above) is the
     keyboard-accessible way to close. role="presentation" already marks the
     backdrop as decoration, so only the click-without-keydown warning needs
     silencing here. -->
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
              inputmode="numeric"
              min="1"
              step="1"
              placeholder="No limit"
              bind:value={amount}
              oninput={commitRate}
              aria-label="Maximum download speed"
            />
            <select
              class="unit-select"
              bind:value={unit}
              onchange={commitRate}
              aria-label="Speed limit unit"
            >
              {#each RATE_UNITS as u (u.value)}
                <option value={u.value}>{u.label}</option>
              {/each}
            </select>
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
          <span class="row-label">yt-dlp</span>
          <span class="path">{ytDlpVersion || 'not installed'}</span>
          {#if updateAvailable}
            <button
              class="primary"
              onclick={onUpdateYtDlp}
              disabled={updating || busy}
              title={busy && !updating
                ? 'Wait for the current download to finish'
                : `Download yt-dlp ${latestVersion}`}
            >
              {updating ? 'Updating…' : `Update to ${latestVersion}`}
            </button>
          {:else}
            <button
              onclick={onCheckForUpdate}
              disabled={checking || updating}
              title="See whether a newer yt-dlp release is available"
            >
              {checking ? 'Checking…' : 'Check for updates'}
            </button>
          {/if}
        </div>
        {#if updateStatus}
          <p class="row-status" role="status">
            {updateStatus}
            {#if changelogUrl}
              <button class="changelog-link" onclick={onOpenChangelog}>
                View changelog
              </button>
            {/if}
          </p>
        {/if}
        <p class="setting-hint">
          Video sites change constantly and yt-dlp ships fixes just as often —
          if downloads have started failing, try updating first.
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
      {:else if activeTab === 'general'}
        <h3 class="pane-title">General</h3>

        <div class="setting">
          <div class="field-head">
            <span class="field-label">Reset Settings</span>
          </div>
          <div class="field-row">
            {#if resetArmed}
              <button class="danger" onclick={confirmReset}>
                Click again to confirm
              </button>
              <button onclick={() => (resetArmed = false)}>Cancel</button>
            {:else}
              <button onclick={armReset}>Reset all settings</button>
            {/if}
          </div>
          <p class="setting-hint">
            Restores every saved setting to its default
          </p>
          {#if resetDone}
            <p class="row-status" role="status">Settings restored to defaults.</p>
          {/if}
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

  .unit-select {
    width: 6rem;
    flex-shrink: 0;
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

  /* Result line under a row's button (e.g. the outcome of an update). */
  .row-status {
    margin: 0.1rem 0 0;
    font-size: var(--fs-sm);
    color: var(--text-dim);
  }

  /* Text-style button: opts out of the global push-button look so the
     changelog reads as a link sitting inside the status line. */
  .changelog-link {
    background: none;
    border: none;
    padding: 0;
    margin-left: 0.4rem;
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
    font-size: inherit;
    font-weight: inherit;
  }

  .changelog-link:hover {
    background: none;
    color: var(--accent-strong);
  }
</style>

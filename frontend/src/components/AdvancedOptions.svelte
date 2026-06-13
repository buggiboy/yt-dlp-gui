<script lang="ts">
  // SponsorBlock segment categories exposed by the SponsorBlock API.
  // Labels match what the SponsorBlock browser extension uses.
  const SB_CATEGORIES = [
    { id: 'sponsor',         label: 'Sponsor' },
    { id: 'intro',           label: 'Intro' },
    { id: 'outro',           label: 'Outro' },
    { id: 'selfpromo',       label: 'Self-promo' },
    { id: 'interaction',     label: 'Interaction' },
    { id: 'music_offtopic',  label: 'Music (off-topic)' },
    { id: 'preview',         label: 'Preview' },
    { id: 'filler',          label: 'Filler' },
  ] as const

  let {
    folder = $bindable(),
    defaultFolder,
    subtitles = $bindable(),
    subLangs = $bindable(),
    embedMeta = $bindable(),
    sponsorBlock = $bindable(),
    sponsorBlockCategories = $bindable(),
    ffmpegAvailable,
    busy,
    onChooseFolder,
    onOpenDepsFolder,
  }: {
    folder: string
    defaultFolder: string
    subtitles: boolean
    subLangs: string
    embedMeta: boolean
    sponsorBlock: boolean
    sponsorBlockCategories: string[]
    ffmpegAvailable: boolean
    busy: boolean
    onChooseFolder: () => void
    onOpenDepsFolder: () => void
  } = $props()

  // Subtitle language presets. "all" tells yt-dlp to grab every available
  // subtitle track. "custom" is a sentinel that reveals the free-text field.
  const LANG_PRESETS = [
    { id: 'en',     label: 'English' },
    { id: 'es',     label: 'Spanish' },
    { id: 'fr',     label: 'French' },
    { id: 'de',     label: 'German' },
    { id: 'ja',     label: 'Japanese' },
    { id: 'ko',     label: 'Korean' },
    { id: 'zh',     label: 'Chinese' },
    { id: 'pt',     label: 'Portuguese' },
    { id: 'all',    label: 'All available' },
    { id: 'custom', label: 'Custom…' },
  ] as const

  const PRESET_IDS = LANG_PRESETS.filter((p) => p.id !== 'custom').map((p) => p.id)

  // Local state: which preset is selected. Initialised from the incoming
  // subLangs value so the dropdown reflects any existing setting.
  let subLangPreset = $state(
    (PRESET_IDS as readonly string[]).includes(subLangs) ? subLangs : 'custom'
  )

  function onLangPresetChange(id: string) {
    subLangPreset = id
    if (id === 'custom') {
      // Clear subLangs so the placeholder guides the user rather than showing
      // a confusing carry-over value from the previously selected preset.
      subLangs = ''
    } else {
      subLangs = id
    }
  }

  let showAdvanced = $state(false)
  let displayPath = $derived(folder || defaultFolder || 'Downloads')

  function toggleSbCategory(id: string, checked: boolean) {
    // Reassign (don't mutate) so Svelte tracks the change through $bindable.
    if (checked) {
      sponsorBlockCategories = [...sponsorBlockCategories, id]
    } else {
      sponsorBlockCategories = sponsorBlockCategories.filter((c) => c !== id)
    }
  }
</script>

<div class="advanced">
  <button class="toggle" onclick={() => (showAdvanced = !showAdvanced)}>
    {showAdvanced ? '▾' : '▸'} Advanced options
  </button>

  {#if showAdvanced}
    <div class="body">

      <!-- Download folder -->
      <div class="row">
        <span class="row-label">Download folder</span>
        <span class="path" title={displayPath}>{displayPath}</span>
        {#if folder}
          <button onclick={() => (folder = '')} disabled={busy} title="Reset to Downloads folder">
            Reset
          </button>
        {/if}
        <button
          class="icon-btn"
          onclick={onChooseFolder}
          disabled={busy}
          title="Choose a folder…"
          aria-label="Choose download folder"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M1 3.5A1.5 1.5 0 0 1 2.5 2h3.1c.4 0 .78.16 1.06.44L7.7 3.5h5.8A1.5 1.5 0 0 1 15 5v7.5A1.5 1.5 0 0 1 13.5 14h-11A1.5 1.5 0 0 1 1 12.5v-9z" />
          </svg>
        </button>
      </div>

      <!-- Embed metadata & thumbnail -->
      <div class="row">
        <label class="check-label">
          <input type="checkbox" bind:checked={embedMeta} disabled={busy} />
          Embed metadata &amp; thumbnail
        </label>
        {#if embedMeta && !ffmpegAvailable}
          <span class="note">thumbnail requires ffmpeg</span>
        {/if}
      </div>

      <!-- Subtitles -->
      <div class="row" class:wrap={subtitles && subLangPreset === 'custom'}>
        <label class="check-label">
          <input type="checkbox" bind:checked={subtitles} disabled={busy} />
          Download subtitles
        </label>
        {#if subtitles}
          <label class="inline-label">
            Language
            <select
              value={subLangPreset}
              disabled={busy}
              onchange={(e) => onLangPresetChange(e.currentTarget.value)}
            >
              {#each LANG_PRESETS as p (p.id)}
                <option value={p.id}>{p.label}</option>
              {/each}
            </select>
          </label>
          {#if subLangPreset === 'custom'}
            <!-- Free-text field for power users. yt-dlp accepts BCP-47 codes
                 comma-separated, e.g. "en,fr" or "en.*" for all English variants. -->
            <input
              class="lang-input"
              type="text"
              bind:value={subLangs}
              disabled={busy}
              placeholder="en,fr"
              autocomplete="off"
              title="Comma-separated BCP-47 language codes, e.g. en,fr,de"
            />
          {/if}
        {/if}
      </div>

      <!-- SponsorBlock -->
      <div class="row" class:wrap={sponsorBlock}>
        <label class="check-label">
          <input type="checkbox" bind:checked={sponsorBlock} disabled={busy} />
          SponsorBlock
        </label>
        {#if sponsorBlock}
          <!-- Category checkboxes. At least one should be checked for the
               flag to do anything; the grid makes the list scannable. -->
          <div class="sb-grid">
            {#each SB_CATEGORIES as cat (cat.id)}
              <label class="sb-cat">
                <input
                  type="checkbox"
                  checked={sponsorBlockCategories.includes(cat.id)}
                  disabled={busy}
                  onchange={(e) => toggleSbCategory(cat.id, e.currentTarget.checked)}
                />
                {cat.label}
              </label>
            {/each}
          </div>
          {#if sponsorBlockCategories.length === 0}
            <span class="note">select at least one category</span>
          {/if}
        {/if}
      </div>

      <!-- Open dependencies folder -->
      <div class="row">
        <span class="row-label">Dependencies folder</span>
        <span class="path">where Python, yt-dlp &amp; ffmpeg are stored</span>
        <button onclick={onOpenDepsFolder} title="Open the folder where the app's dependencies are stored">
          Open
        </button>
      </div>

    </div>
  {/if}
</div>

<style>
  .advanced {
    margin-top: 1rem;
    text-align: left;
  }

  .toggle {
    font-size: 0.85rem;
  }

  /* Same card surface as ClipSection so the two cards read consistently. */
  .body {
    margin-top: 0.5rem;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 0.25rem 1rem;
  }

  /* Each option occupies one row with a subtle top border separating it from
     the previous one. `wrap` is set when extra content (lang field, SB
     categories) appears below the main control. */
  .row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.07);
    font-size: 0.9rem;
    flex-wrap: nowrap;
  }

  .row:first-child {
    border-top: none;
  }

  .row.wrap {
    flex-wrap: wrap;
    row-gap: 0.5rem;
  }

  /* Static text label (used for download folder row). */
  .row-label {
    white-space: nowrap;
  }

  /* The folder path shrinks with an ellipsis; full path in title tooltip. */
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 0.85rem;
    opacity: 0.65;
  }

  /* Checkbox + inline label. The <label> wraps the <input> so the whole line
     is clickable, matching how native macOS checkboxes behave. */
  .check-label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
  }

  /* Secondary inline label (e.g. "Languages [input]") that appears on the
     same row as the checkbox when the option is enabled. */
  .inline-label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    margin-left: auto;
  }

  .lang-input {
    font-size: 0.85rem;
    padding: 3px 7px;
    border-radius: 5px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background-color: rgba(255, 255, 255, 0.08);
    color: #eee;
    outline: none;
    width: 80px;
  }

  .lang-input:focus {
    border-color: #4f9dff;
  }

  /* SponsorBlock category grid. Two columns keeps it compact; each cell is a
     native checkbox + label. */
  .sb-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.3rem 1rem;
    width: 100%;
    margin-top: 0.1rem;
  }

  .sb-cat {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .note {
    font-size: 0.78rem;
    opacity: 0.5;
    margin-left: auto;
  }

  /* Icon-only button: center the SVG without overriding native button look. */
  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .icon-btn svg {
    display: block;
  }
</style>

<script lang="ts">
  import { untrack } from 'svelte'
  import type { FilenameFormat, CleanupPresetId } from '../lib/filename/types'
  import { TOKENS, tokenByField, CATEGORY_LABELS, type TokenCategory } from '../lib/filename/tokens'
  import { CLEANUP_PRESETS, validatePattern } from '../lib/filename/cleanup'
  import { compile } from '../lib/filename/compile'
  import { preview } from '../lib/filename/preview'
  import {
    NAMED_PRESETS,
    matchPreset,
    cloneFormat,
    tokenSeg,
    literalSeg,
    uid,
  } from '../lib/filename/presets'

  let {
    format,
    onApply,
    onCancel,
  }: {
    format: FilenameFormat
    onApply: (f: FilenameFormat) => void
    onCancel: () => void
  } = $props()

  // Work on a detached draft so Cancel discards cleanly. The component is
  // mounted fresh each time the modal opens (keyed by an {#if} in App.svelte),
  // so snapshotting `format` once at construction is intentional — untrack
  // makes that explicit and avoids the reactive-capture hint.
  let draft = $state(untrack(() => cloneFormat(format)))

  let selectedSegId = $state<string | null>(null)
  let showAdvanced = $state(false)
  let showCustomRules = $state(false)

  let previewRes = $derived(preview(draft))
  let compiled = $derived(compile(draft))
  let presetValue = $derived(matchPreset(draft))
  let selectedToken = $derived(
    draft.segments.find((s) => s.id === selectedSegId && s.kind === 'token')
  )

  // Tokens grouped by category for the "add field" menu.
  const TOKEN_GROUPS = Object.keys(CATEGORY_LABELS).map((cat) => ({
    label: CATEGORY_LABELS[cat as TokenCategory],
    tokens: TOKENS.filter((t) => t.category === cat),
  }))

  const DATE_FORMATS = [
    { v: '%Y-%m-%d', l: '2009-10-25' },
    { v: '%Y%m%d', l: '20091025' },
    { v: '%Y-%m', l: '2009-10' },
    { v: '%Y', l: '2009' },
  ]

  function applyPreset(id: string) {
    const p = NAMED_PRESETS.find((x) => x.id === id)
    if (!p) return
    // Replace only the layout; keep the user's advanced cleanup settings.
    draft.segments = p.build().segments
    selectedSegId = null
  }

  function addToken(field: string) {
    if (!field) return
    const seg = tokenSeg(field)
    draft.segments = [...draft.segments, seg]
    selectedSegId = seg.id
  }

  function addLiteral() {
    const seg = literalSeg(' - ')
    draft.segments = [...draft.segments, seg]
    selectedSegId = null
  }

  function removeSeg(id: string) {
    draft.segments = draft.segments.filter((s) => s.id !== id)
    if (selectedSegId === id) selectedSegId = null
  }

  function toggleChip(id: string) {
    selectedSegId = selectedSegId === id ? null : id
  }

  function isCleanupOn(id: CleanupPresetId): boolean {
    return draft.cleanupPresets.includes(id)
  }

  function toggleCleanup(id: CleanupPresetId, on: boolean) {
    draft.cleanupPresets = on
      ? [...draft.cleanupPresets, id]
      : draft.cleanupPresets.filter((c) => c !== id)
  }

  function addRule() {
    draft.customRules = [
      ...draft.customRules,
      { id: uid(), field: 'title', pattern: '', replace: '' },
    ]
    showCustomRules = true
  }

  function removeRule(id: string) {
    draft.customRules = draft.customRules.filter((r) => r.id !== id)
  }

  function apply() {
    onApply(cloneFormat(draft))
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape') onCancel() }} />

<!-- Backdrop dismissal is a mouse convenience; Escape (handled above) is the
     keyboard-accessible way to close, so the static-element interaction here is
     intentional. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="backdrop" role="presentation" onclick={onCancel}>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    aria-labelledby="fnmodal-title"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
  >
    <h2 id="fnmodal-title">Filename format</h2>
    <p class="sub">Build the name your downloads are saved as. The extension is added automatically.</p>

    <!-- Preset starting points -->
    <div class="field-row">
      <label class="lbl" for="fn-preset">Start from</label>
      <select id="fn-preset" value={presetValue} onchange={(e) => applyPreset(e.currentTarget.value)}>
        {#if presetValue === 'custom'}
          <option value="custom">Custom</option>
        {/if}
        {#each NAMED_PRESETS as p (p.id)}
          <option value={p.id}>{p.label}</option>
        {/each}
      </select>
    </div>

    <!-- Chip builder -->
    <div class="builder">
      {#each draft.segments as seg (seg.id)}
        {#if seg.kind === 'token'}
          <span class="chip" class:active={seg.id === selectedSegId}>
            <button class="chip-main" onclick={() => toggleChip(seg.id)} title="Edit options">
              {tokenByField(seg.field)?.label ?? seg.field}
            </button>
            <button class="chip-x" onclick={() => removeSeg(seg.id)} aria-label="Remove field">×</button>
          </span>
        {:else}
          <span class="lit">
            <input
              class="lit-input"
              type="text"
              bind:value={seg.text}
              size={Math.max(seg.text.length, 2)}
              aria-label="Literal text"
              autocomplete="off"
              spellcheck="false"
            />
            <button class="chip-x" onclick={() => removeSeg(seg.id)} aria-label="Remove text">×</button>
          </span>
        {/if}
      {/each}
      <span class="ext" title="Always added — the file extension">.ext</span>
    </div>

    <!-- Add controls -->
    <div class="add-row">
      <select
        class="add-field"
        value=""
        onchange={(e) => { addToken(e.currentTarget.value); e.currentTarget.value = '' }}
      >
        <option value="" disabled selected>+ Add field…</option>
        {#each TOKEN_GROUPS as g (g.label)}
          <optgroup label={g.label}>
            {#each g.tokens as t (t.id)}
              <option value={t.field}>{t.label}</option>
            {/each}
          </optgroup>
        {/each}
      </select>
      <button onclick={addLiteral}>+ Add text</button>
    </div>

    <!-- Token options panel -->
    {#if selectedToken && selectedToken.kind === 'token'}
      {@const def = tokenByField(selectedToken.field)}
      <div class="opts">
        <div class="opts-title">{def?.label ?? selectedToken.field} options</div>
        <div class="opts-grid">
          {#if def?.supportsTruncate}
            <label class="opt">
              Max length
              <input
                type="number"
                min="0"
                placeholder="off"
                bind:value={selectedToken.opts.truncate}
              />
            </label>
          {/if}
          <label class="opt">
            If missing, use
            <input
              type="text"
              placeholder="(leave blank)"
              bind:value={selectedToken.opts.default}
              autocomplete="off"
            />
          </label>
          {#if def?.isDate}
            <label class="opt">
              Date format
              <select bind:value={selectedToken.opts.dateFormat}>
                {#each DATE_FORMATS as d (d.v)}
                  <option value={d.v}>{d.l}</option>
                {/each}
              </select>
            </label>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Live preview -->
    <div class="preview">
      <span class="preview-lbl">Preview</span>
      <code class="preview-name">{previewRes.name}</code>
      {#if previewRes.approximate}
        <span class="approx" title="Regex cleanup is simulated with JavaScript; yt-dlp uses Python regex, so the result is approximate.">≈ approximate</span>
      {/if}
    </div>
    {#if previewRes.warning}
      <p class="warn">{previewRes.warning}</p>
    {/if}

    <!-- Advanced disclosure -->
    <button class="disclose" onclick={() => (showAdvanced = !showAdvanced)}>
      {showAdvanced ? '▾' : '▸'} Advanced
    </button>

    {#if showAdvanced}
      <div class="adv">
        <!-- Tier 1: cleanup toggles -->
        <div class="adv-section">
          <div class="adv-head">Clean up the title</div>
          {#each CLEANUP_PRESETS as p (p.id)}
            <label class="check">
              <input
                type="checkbox"
                checked={isCleanupOn(p.id)}
                onchange={(e) => toggleCleanup(p.id, e.currentTarget.checked)}
              />
              <span>{p.label}</span>
            </label>
          {/each}
        </div>

        <!-- Tier 2: global options -->
        <div class="adv-section">
          <div class="adv-head">Filename safety</div>
          <label class="check">
            <input type="checkbox" bind:checked={draft.restrictFilenames} />
            <span>ASCII-only, no spaces (safe for any drive)</span>
          </label>
          <label class="opt inline">
            Max filename length
            <input type="number" min="0" placeholder="off" bind:value={draft.maxLength} />
          </label>
        </div>

        <!-- Tier 3: custom regex rules -->
        <div class="adv-section">
          <button class="disclose sub" onclick={() => (showCustomRules = !showCustomRules)}>
            {showCustomRules ? '▾' : '▸'} Custom regex rules
            {#if draft.customRules.length}<span class="count">{draft.customRules.length}</span>{/if}
          </button>
          {#if showCustomRules}
            <p class="hint">
              Power-user find/replace on a metadata field, in order. Previews are approximate —
              yt-dlp uses Python regex.
            </p>
            {#each draft.customRules as rule (rule.id)}
              {@const err = rule.pattern.trim() ? validatePattern(rule.pattern) : null}
              <div class="rule">
                <select bind:value={rule.field} aria-label="Field">
                  {#each TOKENS as t (t.id)}
                    <option value={t.field}>{t.field}</option>
                  {/each}
                </select>
                <input
                  class="mono"
                  type="text"
                  bind:value={rule.pattern}
                  placeholder="pattern"
                  aria-label="Pattern"
                  autocomplete="off"
                  spellcheck="false"
                />
                <span class="arrow">→</span>
                <input
                  class="mono"
                  type="text"
                  bind:value={rule.replace}
                  placeholder="replacement"
                  aria-label="Replacement"
                  autocomplete="off"
                  spellcheck="false"
                />
                <button class="chip-x" onclick={() => removeRule(rule.id)} aria-label="Remove rule">×</button>
                {#if err}<span class="rule-err">{err}</span>{/if}
              </div>
            {/each}
            <button onclick={addRule}>+ Add rule</button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Raw template readout for the curious -->
    <p class="raw" title="The yt-dlp output template these settings compile to">
      -o <code>{compiled.outtmpl}</code>
    </p>

    <div class="actions">
      <button onclick={onCancel}>Cancel</button>
      <button class="primary" onclick={apply}>Apply</button>
    </div>
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
    z-index: 20;
  }

  .modal {
    width: min(680px, calc(100vw - 2rem));
    max-height: min(86vh, 760px);
    overflow-y: auto;
    background: #28282a;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    text-align: left;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  h2 {
    margin: 0 0 0.25rem;
    font-size: 1.1rem;
  }

  .sub {
    margin: 0 0 1rem;
    font-size: 0.85rem;
    opacity: 0.6;
    line-height: 1.4;
  }

  .field-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.9rem;
  }

  .lbl {
    font-size: 0.85rem;
    opacity: 0.8;
  }

  /* Chip builder surface */
  .builder {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
    padding: 0.6rem;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.04);
    min-height: 2.4rem;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    background: rgba(79, 157, 255, 0.18);
    border: 1px solid rgba(79, 157, 255, 0.5);
    border-radius: 14px;
    overflow: hidden;
  }

  .chip.active {
    border-color: #4f9dff;
    background: rgba(79, 157, 255, 0.32);
  }

  .chip-main {
    font-size: 0.82rem;
    padding: 3px 4px 3px 10px;
    color: #dce8ff;
    background: none;
    border: none;
    cursor: pointer;
  }

  .chip-x {
    font-size: 0.95rem;
    line-height: 1;
    padding: 2px 7px 2px 3px;
    color: rgba(255, 255, 255, 0.6);
    background: none;
    border: none;
    cursor: pointer;
  }

  .chip-x:hover {
    color: #fff;
  }

  .lit {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }

  .lit-input {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.8rem;
    padding: 3px 6px;
    border-radius: 5px;
    border: 1px dashed rgba(255, 255, 255, 0.25);
    background: rgba(255, 255, 255, 0.06);
    color: #eee;
    outline: none;
    min-width: 1.5rem;
  }

  .lit-input:focus {
    border-color: #4f9dff;
    border-style: solid;
  }

  .ext {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.78rem;
    padding: 3px 9px;
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.07);
    color: rgba(255, 255, 255, 0.45);
  }

  .add-row {
    display: flex;
    gap: 0.5rem;
    margin: 0.6rem 0 0.2rem;
  }

  .add-field {
    max-width: 12rem;
  }

  /* Token options panel */
  .opts {
    margin-top: 0.7rem;
    padding: 0.7rem 0.85rem;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.03);
  }

  .opts-title {
    font-size: 0.8rem;
    opacity: 0.75;
    margin-bottom: 0.5rem;
  }

  .opts-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
  }

  .opt {
    display: inline-flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.78rem;
    opacity: 0.9;
  }

  .opt.inline {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }

  .opt input[type='number'] {
    width: 6rem;
  }

  .opt input[type='text'] {
    width: 9rem;
  }

  /* Preview */
  .preview {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 1rem;
    padding-top: 0.85rem;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    flex-wrap: wrap;
  }

  .preview-lbl {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.5;
  }

  .preview-name {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.9rem;
    color: #eaeaea;
    word-break: break-all;
  }

  .approx {
    font-size: 0.72rem;
    color: #e3b341;
    opacity: 0.85;
    white-space: nowrap;
  }

  .warn {
    margin: 0.35rem 0 0;
    font-size: 0.78rem;
    color: #e3b341;
    opacity: 0.85;
  }

  /* Disclosures */
  .disclose {
    margin-top: 1rem;
    font-size: 0.85rem;
    background: none;
    border: none;
    color: #bcd2f5;
    cursor: pointer;
    padding: 0.2rem 0;
  }

  .disclose.sub {
    margin-top: 0.3rem;
    color: #9fb6da;
  }

  .count {
    display: inline-block;
    margin-left: 0.3rem;
    font-size: 0.72rem;
    background: rgba(79, 157, 255, 0.25);
    border-radius: 8px;
    padding: 0 6px;
  }

  .adv {
    margin-top: 0.4rem;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 0.4rem 0.9rem;
    background: rgba(255, 255, 255, 0.03);
  }

  .adv-section {
    padding: 0.7rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.07);
  }

  .adv-section:first-child {
    border-top: none;
  }

  .adv-head {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    opacity: 0.55;
    margin-bottom: 0.5rem;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.85rem;
    padding: 0.18rem 0;
    cursor: pointer;
  }

  .hint {
    font-size: 0.76rem;
    opacity: 0.55;
    line-height: 1.4;
    margin: 0.1rem 0 0.6rem;
  }

  /* Custom rule rows */
  .rule {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin-bottom: 0.45rem;
  }

  .rule select {
    max-width: 8rem;
  }

  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.8rem;
    padding: 4px 7px;
    border-radius: 5px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background: rgba(255, 255, 255, 0.08);
    color: #eee;
    outline: none;
    flex: 1;
    min-width: 6rem;
  }

  .mono:focus {
    border-color: #4f9dff;
  }

  .arrow {
    opacity: 0.5;
    font-size: 0.85rem;
  }

  .rule-err {
    flex-basis: 100%;
    font-size: 0.74rem;
    color: #ff8a8a;
  }

  .raw {
    margin: 1rem 0 0;
    font-size: 0.72rem;
    opacity: 0.45;
    word-break: break-all;
  }

  .raw code {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1.1rem;
  }

  .primary {
    color: #fff;
    background: rgba(79, 157, 255, 0.85);
    border: 1px solid rgba(79, 157, 255, 0.9);
    border-radius: 6px;
    padding: 4px 14px;
  }
</style>

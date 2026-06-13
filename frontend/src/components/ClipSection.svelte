<script lang="ts">
  import { isValidTime } from '../lib/time'
  import InfoTip from './InfoTip.svelte'

  let {
    clipStart = $bindable(),
    clipEnd = $bindable(),
    ffmpegAvailable,
    busy,
    onRequestFfmpeg,
  }: {
    clipStart: string
    clipEnd: string
    ffmpegAvailable: boolean
    busy: boolean
    onRequestFfmpeg: () => void
  } = $props()

  let startValid = $derived(isValidTime(clipStart))
  let endValid = $derived(isValidTime(clipEnd))
</script>

<div
  class="clip"
  class:disabled={!ffmpegAvailable}
  role="presentation"
  onclick={() => { if (!ffmpegAvailable && !busy) onRequestFfmpeg() }}
>
  <span class="title">Clip</span>

  <InfoTip label="About clipping">
    Leave both blank to download the whole video. Use seconds (90), MM:SS (1:30),
    or HH:MM:SS — leave one side blank to clip from the beginning or to the end.
  </InfoTip>

  <div class="time-group">
    <label class="time-field">
      <span>Start</span>
      <input
        class="time-input"
        class:invalid={!startValid}
        type="text"
        placeholder="0:30"
        bind:value={clipStart}
        disabled={busy || !ffmpegAvailable}
        autocomplete="off"
      />
    </label>
    <span class="dash">–</span>
    <label class="time-field">
      <span>Stop</span>
      <input
        class="time-input"
        class:invalid={!endValid}
        type="text"
        placeholder="1:45"
        bind:value={clipEnd}
        disabled={busy || !ffmpegAvailable}
        autocomplete="off"
      />
    </label>
  </div>

  {#if !ffmpegAvailable}
    <span class="ffmpeg-notice">requires ffmpeg — click to install</span>
  {/if}
</div>

<style>
  .clip {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    text-align: left;
    transition: opacity 0.15s ease;
  }

  .clip.disabled {
    opacity: 0.55;
    cursor: pointer;
  }

  .clip.disabled:hover {
    opacity: 0.75;
  }

  .title {
    font-size: var(--fs-base);
    font-weight: 600;
  }

  .time-group {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .time-field {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: var(--fs-sm);
    color: var(--text-dim);
  }

  /* Inherits the shared input look; only the clip-specific sizing differs. */
  .time-input {
    padding: 6px 9px;
    width: 66px;
    font-variant-numeric: tabular-nums;
    text-align: center;
  }

  .time-input.invalid {
    border-color: #e5534b;
  }

  .dash {
    opacity: 0.5;
  }

  .ffmpeg-notice {
    font-size: var(--fs-xs);
    color: #f0b429;
  }
</style>

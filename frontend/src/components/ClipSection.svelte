<script lang="ts">
  import { isValidTime } from '../lib/time'

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
  <div class="header">
    <span class="title">Clip</span>
    {#if !ffmpegAvailable}
      <span class="ffmpeg-notice">requires ffmpeg — click to install</span>
    {/if}
  </div>

  <div class="time-row">
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

  <p class="hint">
    Leave both blank to download the whole video. Use seconds (90), MM:SS (1:30),
    or HH:MM:SS — leave one side blank to clip from the beginning or to the end.
  </p>
</div>

<style>
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

  .header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .title {
    font-size: 0.9rem;
    font-weight: 600;
  }

  .ffmpeg-notice {
    font-size: 0.78rem;
    color: #f0b429;
    margin-left: auto;
  }

  .time-row {
    display: flex;
    align-items: flex-end;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .time-field {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
    font-size: 0.8rem;
    opacity: 0.9;
  }

  .time-input {
    font-size: 1rem;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background-color: rgba(255, 255, 255, 0.08);
    color: #eee;
    outline: none;
    width: 90px;
    font-variant-numeric: tabular-nums;
  }

  .time-input:focus {
    border-color: #4f9dff;
  }

  .time-input.invalid {
    border-color: #e5534b;
  }

  .dash {
    padding-bottom: 6px;
    opacity: 0.5;
  }

  .hint {
    margin-top: 0.5rem;
    margin-bottom: 0;
    font-size: 0.78rem;
    opacity: 0.5;
  }
</style>

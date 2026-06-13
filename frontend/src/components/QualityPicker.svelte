<script lang="ts">
  import type { VideoFormat } from '../lib/types'

  let {
    quality = $bindable(),
    audioFormat = $bindable(),
    formatOptions,
    audioExtension,
    loadingFormats,
    availableFormats,
    ffmpegAvailable,
    busy,
  }: {
    quality: string
    audioFormat: string
    formatOptions: VideoFormat[]
    audioExtension: string
    loadingFormats: boolean
    availableFormats: VideoFormat[] | null
    ffmpegAvailable: boolean
    busy: boolean
  } = $props()

  function qualityLabel(format: VideoFormat): string {
    let label = `${format.height}p`
    if (format.height >= 4320) label += ' (8K)'
    else if (format.height >= 2160) label += ' (4K)'
    if (format.ext) label += ` · ${format.ext}`
    return label
  }
</script>

<div class="quality">
  <label>
    <span>Quality</span>
    <select bind:value={quality} disabled={busy}>
      <option value="best">Best available</option>
      {#each formatOptions as format (format.height)}
        <option value={String(format.height)}>{qualityLabel(format)}</option>
      {/each}
      <option value="audio">Audio only{audioExtension ? ` · ${audioExtension}` : ''}</option>
    </select>
  </label>

  {#if quality === 'audio'}
    <label>
      <span>Format</span>
      <select
        bind:value={audioFormat}
        disabled={busy || !ffmpegAvailable}
        title={ffmpegAvailable ? undefined : 'Converting audio requires ffmpeg'}
      >
        <option value="">Default{audioExtension ? ` · ${audioExtension}` : ''}</option>
        <option value="mp3">MP3</option>
        <option value="m4a">M4A (AAC)</option>
        <option value="opus">Opus</option>
        <option value="flac">FLAC</option>
      </select>
    </label>
    {#if !ffmpegAvailable}
      <span class="note">conversion requires ffmpeg</span>
    {/if}
  {/if}

  {#if loadingFormats}
    <span class="note">checking available qualities…</span>
  {:else if availableFormats && availableFormats.length > 0}
    <span class="note ok">showing this video's qualities</span>
  {/if}
</div>

<style>
  .quality {
    margin-top: 1rem;
    text-align: left;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  label {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
  }

  label select {
    font-size: 0.9rem;
  }

  .note {
    font-size: 0.78rem;
    opacity: 0.5;
  }

  .note.ok {
    color: #5dc06b;
    opacity: 0.8;
  }
</style>

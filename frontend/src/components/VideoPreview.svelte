<script lang="ts">
  import type { VideoPreviewData } from '../lib/types'

  let {
    previewUrl,
    preview,
    loading,
  }: {
    previewUrl: string | null
    preview: VideoPreviewData | null
    loading: boolean
  } = $props()
</script>

{#if previewUrl}
  {#key previewUrl}
    <div class="preview">
      <iframe
        src={previewUrl}
        title="Video preview"
        allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        allowfullscreen
      ></iframe>
    </div>
  {/key}
{:else if preview?.kind === 'embed'}
  {#key preview.url}
    <div class="preview">
      <iframe
        src={preview.url}
        title={preview.title || 'Video preview'}
        allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        allowfullscreen
      ></iframe>
    </div>
  {/key}
{:else if preview?.kind === 'thumbnail'}
  <div class="preview thumbnail">
    <img src={preview.thumbnail} alt={preview.title || 'Video thumbnail'} />
    {#if preview.title || preview.duration}
      <div class="caption">
        <span class="caption-title">{preview.title}</span>
        {#if preview.duration}
          <span class="caption-duration">{preview.duration}</span>
        {/if}
      </div>
    {/if}
  </div>
{:else if loading}
  <p class="loading">Looking up preview…</p>
{/if}

<style>
  .preview {
    margin-top: 1rem;
    aspect-ratio: 16 / 9;
    border-radius: 10px;
    overflow: hidden;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.1);
  }

  .preview iframe {
    width: 100%;
    height: 100%;
    border: 0;
    display: block;
  }

  .preview.thumbnail {
    position: relative;
  }

  .preview.thumbnail img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .caption {
    position: absolute;
    inset: auto 0 0 0;
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 1.25rem 0.75rem 0.5rem;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.75), transparent);
    text-align: left;
  }

  .caption-title {
    font-size: 0.85rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .caption-duration {
    font-size: 0.8rem;
    opacity: 0.8;
    font-variant-numeric: tabular-nums;
  }

  .loading {
    margin-top: 1rem;
    font-size: 0.85rem;
    opacity: 0.5;
  }
</style>

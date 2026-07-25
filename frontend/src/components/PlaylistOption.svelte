<script lang="ts">
  // Shown only when the URL looks like it points into a playlist. Downloading
  // a few hundred videos is a big enough difference in outcome that it gets an
  // explicit opt-in rather than being inferred from the link.
  import InfoTip from './InfoTip.svelte'

  let {
    checked = $bindable(),
    busy,
  }: {
    checked: boolean
    busy: boolean
  } = $props()
</script>

<div class="playlist" class:on={checked}>
  <label class="check-label">
    <input type="checkbox" bind:checked disabled={busy} />
    Download the whole playlist
  </label>

  <InfoTip label="About playlist downloads" align="left" width={280}>
    This link points into a playlist. By default only the single video it names
    is downloaded — tick this to fetch every video in the list instead.
    <br /><br />
    The quality and clip options describe the first video; yt-dlp picks the
    closest available quality for each of the others, and clipping is applied to
    every one of them.
  </InfoTip>
</div>

<style>
  .playlist {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.6rem;
    padding: 0.45rem 0.7rem;
    text-align: left;
    font-size: var(--fs-base);
    background: var(--surface);
    border: 1px solid var(--surface-border);
    border-radius: var(--radius);
    transition: border-color 0.12s ease;
  }

  /* Ticked is the consequential state, so give it a visible edge — this is the
     difference between one file and several hundred. */
  .playlist.on {
    border-color: var(--accent);
  }

  /* Whole line clickable, matching the other checkbox rows in the app. */
  .check-label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
  }
</style>

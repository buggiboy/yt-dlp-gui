<script lang="ts">
  import { dependencyKeys, dependencyInfo, type DependencyKey } from '../lib/deps'

  let {
    deps,
    busy,
    onRequestInstall,
    onSkipFfmpeg,
  }: {
    deps: Record<string, boolean>
    busy: boolean
    onRequestInstall: (key: DependencyKey) => void
    onSkipFfmpeg: () => void
  } = $props()
</script>

<div class="setup">
  <p class="title">A few components are needed before downloading videos:</p>
  {#each dependencyKeys as key (key)}
    {@const info = dependencyInfo[key]}
    <div class="row">
      <div class="text">
        <span class="name">
          {info.name}
          <span class="size">({info.size})</span>
          {#if info.optional}<span class="optional-badge">optional</span>{/if}
        </span>
        <span class="description">{info.description}</span>
      </div>
      {#if deps[key]}
        <span class="installed">✓ Installed</span>
      {:else}
        <div class="actions">
          <button
            onclick={() => onRequestInstall(key)}
            disabled={busy || (key === 'extras' && !deps.python)}
            title={key === 'extras' && !deps.python ? 'Requires the Python runtime' : undefined}
          >
            Download…
          </button>
          {#if key === 'ffmpeg'}
            <button onclick={onSkipFfmpeg} disabled={busy}>Skip</button>
          {/if}
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .setup {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.12);
    padding: 0.75rem 1rem 0.5rem;
    border-radius: 8px;
    margin-bottom: 1.25rem;
    text-align: left;
  }

  .title {
    font-size: 0.9rem;
    margin: 0.25rem 0 0.5rem;
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.5rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .name {
    font-size: 0.9rem;
    font-weight: 600;
  }

  .size {
    font-weight: 400;
    opacity: 0.55;
  }

  .description {
    font-size: 0.8rem;
    opacity: 0.6;
  }

  .installed {
    font-size: 0.85rem;
    color: #5dc06b;
    white-space: nowrap;
  }

  .actions {
    display: flex;
    gap: 0.4rem;
    white-space: nowrap;
  }

  .optional-badge {
    font-size: 0.65rem;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.55;
    border: 1px solid rgba(255, 255, 255, 0.3);
    border-radius: 999px;
    padding: 0.1rem 0.45rem;
    vertical-align: middle;
  }
</style>

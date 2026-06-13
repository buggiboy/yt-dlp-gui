<script lang="ts">
  let {
    busy,
    percent,
    speed,
    eta,
    step,
  }: {
    busy: boolean
    percent: number
    speed: string
    eta: string
    step: string
  } = $props()
</script>

{#if busy || percent > 0}
  <div
    class="progress"
    role="progressbar"
    aria-valuenow={percent}
    aria-valuemin="0"
    aria-valuemax="100"
  >
    <div class="fill" style="width: {percent}%"></div>
  </div>
  <p class="meta">
    {percent.toFixed(1)}%{speed ? ` · ${speed}` : ''}{eta ? ` · ETA ${eta}` : ''}
  </p>
  {#if step}
    <p class="step">{step}</p>
  {/if}
{/if}

<style>
  .progress {
    margin-top: 1.5rem;
    height: 10px;
    width: 100%;
    background: rgba(255, 255, 255, 0.12);
    border-radius: 999px;
    overflow: hidden;
  }

  .fill {
    height: 100%;
    background: linear-gradient(to right, #4f9dff, #8ad);
    border-radius: 999px;
    transition: width 0.2s ease;
  }

  .meta {
    margin-top: 0.5rem;
    font-size: 0.85rem;
    opacity: 0.8;
    font-variant-numeric: tabular-nums;
  }

  .step {
    margin-top: 0.25rem;
    font-size: 0.8rem;
    opacity: 0.6;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>

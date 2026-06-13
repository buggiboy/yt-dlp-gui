<script lang="ts">
  // Reusable "i" badge with a hover/focus tooltip. The tooltip content is
  // passed as children so each call site supplies its own copy. Width and
  // horizontal alignment are configurable for tight layouts.
  import type { Snippet } from 'svelte'

  let {
    label,
    width = 240,
    align = 'center',
    children,
  }: {
    /** Accessible label for the trigger, e.g. "About SponsorBlock". */
    label: string
    /** Tooltip box width in px. */
    width?: number
    /** Horizontal anchoring of the tooltip relative to the badge. */
    align?: 'left' | 'center' | 'right'
    children: Snippet
  } = $props()
</script>

<button
  type="button"
  class="info"
  aria-label={label}
  onclick={(e) => e.stopPropagation()}
>
  i
  <span
    class="tooltip {align}"
    role="tooltip"
    style="width: {width}px;"
  >
    {@render children()}
  </span>
</button>

<style>
  .info {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 15px;
    height: 15px;
    padding: 0;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.4);
    background: none;
    color: rgba(255, 255, 255, 0.7);
    font-size: 0.68rem;
    font-style: italic;
    font-weight: 600;
    line-height: 1;
    cursor: help;
    flex-shrink: 0;
  }

  .info:hover,
  .info:focus-visible {
    background: none;
    border-color: rgba(255, 255, 255, 0.7);
    color: #eee;
    outline: none;
  }

  .tooltip {
    position: absolute;
    bottom: calc(100% + 8px);
    padding: 0.5rem 0.65rem;
    border-radius: var(--radius-control);
    background: #1f1f1f;
    border: 1px solid rgba(255, 255, 255, 0.15);
    color: #ddd;
    font-size: var(--fs-sm);
    font-style: normal;
    font-weight: 400;
    line-height: 1.4;
    text-align: left;
    white-space: normal;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.12s ease;
    z-index: 20;
    pointer-events: none;
  }

  /* Horizontal anchoring. The arrow tracks the badge in every case. */
  .tooltip.center {
    left: 50%;
    transform: translateX(-50%);
  }
  .tooltip.left {
    left: 0;
  }
  .tooltip.right {
    right: 0;
  }

  .tooltip::after {
    content: '';
    position: absolute;
    top: 100%;
    border: 5px solid transparent;
    border-top-color: #1f1f1f;
  }
  .tooltip.center::after {
    left: 50%;
    transform: translateX(-50%);
  }
  .tooltip.left::after {
    left: 5px;
  }
  .tooltip.right::after {
    right: 5px;
  }

  .info:hover .tooltip,
  .info:focus-visible .tooltip {
    opacity: 1;
    visibility: visible;
  }
</style>

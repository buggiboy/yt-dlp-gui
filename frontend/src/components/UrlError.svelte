<script lang="ts">
  import type { UrlProblem } from '../lib/types'

  let {
    problem,
    checking,
    overridden = false,
    onOpenSupportedSites,
  }: {
    problem: UrlProblem | null
    checking: boolean
    /** True when the user's own cookies make a sign-in verdict advisory rather
     *  than fatal, so this reads as a heads-up and the Download button stays
     *  live. */
    overridden?: boolean
    /** Opens yt-dlp's supported-sites list in the user's browser. */
    onOpenSupportedSites: () => void
  } = $props()

  // Only offer the supported-sites list where it's actually the next step.
  // A private video or a 404 has nothing to do with site support, and pointing
  // at a 1,800-entry list would just be noise.
  let showSitesLink = $derived(
    problem?.kind === 'unsupported' || problem?.kind === 'novideo'
  )

  // "Needs an account" is the one verdict the user can do something about from
  // inside the app, so say what that something is — but only while they haven't
  // already done it.
  let showCookieHint = $derived(problem?.kind === 'auth' && !overridden)
</script>

{#if problem}
  <p class="url-message" class:error={!overridden} class:warn={overridden} role="alert">
    <svg
      class="icon"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
    <span>
      {problem.message}
      {#if showSitesLink}
        <button class="sites-link" onclick={onOpenSupportedSites}>
          See supported sites
        </button>
      {/if}
      {#if showCookieHint}
        <span class="hint">
          Add <code>--cookies-from-browser chrome</code> under Advanced options →
          Extra arguments to download it with your browser's login.
        </span>
      {:else if overridden}
        <span class="hint">
          Trying anyway — the check ran without the cookies you supplied.
        </span>
      {/if}
    </span>
  </p>
{:else if checking}
  <p class="url-message checking">
    <svg
      class="icon spin"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      aria-hidden="true"
    >
      <path d="M21 12a9 9 0 1 1-6.22-8.56" />
    </svg>
    <span>Checking link…</span>
  </p>
{/if}

<style>
  /* Occupies the same slot as the "Saves as" line, so swapping between them
     doesn't shift the controls below. */
  .url-message {
    display: flex;
    align-items: flex-start;
    gap: 0.4rem;
    margin: 0.5rem 0 0;
    padding: 0 0.1rem;
    text-align: left;
    font-size: var(--fs-sm);
    line-height: 1.4;
  }

  .error {
    color: var(--danger-text);
  }

  /* Advisory rather than fatal: the download is still allowed, so amber rather
     than red. */
  .warn {
    color: #f0b429;
  }

  .checking {
    color: var(--text-faint);
  }

  /* Follow-up guidance under a message. Dimmer than the message itself so the
     verdict still reads first. */
  .hint {
    display: block;
    margin-top: 0.15rem;
    color: var(--text-dim);
  }

  .hint code {
    font-family: var(--mono);
    font-size: 0.95em;
    background: var(--control-bg);
    border-radius: 3px;
    padding: 0 0.25em;
  }

  .icon {
    flex-shrink: 0;
    /* Nudge down so the circle centres on the first line of text rather than
       sitting on its cap height. */
    margin-top: 0.15em;
  }

  .spin {
    animation: spin 0.9s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* Text-style button: opts out of the global push-button look. */
  .sites-link {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
    font-size: inherit;
    font-weight: inherit;
  }

  .sites-link:hover {
    background: none;
    color: var(--accent-strong);
  }

  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation: none;
    }
  }
</style>

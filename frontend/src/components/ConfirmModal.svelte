<script lang="ts">
  import { dependencyInfo, type DependencyKey } from '../lib/deps'

  let {
    target,
    onConfirm,
    onCancel,
  }: {
    target: DependencyKey
    onConfirm: () => void
    onCancel: () => void
  } = $props()

  let info = $derived(dependencyInfo[target])
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
    aria-labelledby="modal-title"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
  >
    <h2 id="modal-title">Download {info.name}?</h2>
    <p class="details">{info.details}</p>
    <p class="source">
      Download size: {info.size}<br />
      Source: <strong>{info.source}</strong>
    </p>
    <div class="actions">
      <button onclick={onCancel}>Cancel</button>
      <button class="primary" onclick={onConfirm}>Download</button>
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
    z-index: 10;
  }

  .modal {
    width: min(440px, calc(100vw - 3rem));
    background: #28282a;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    text-align: left;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  .modal h2 {
    margin: 0 0 0.75rem;
    font-size: 1.1rem;
  }

  .details {
    font-size: 0.88rem;
    line-height: 1.45;
    opacity: 0.85;
    margin: 0 0 0.75rem;
  }

  .source {
    font-size: 0.85rem;
    opacity: 0.8;
    margin: 0 0 1rem;
    line-height: 1.5;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
</style>

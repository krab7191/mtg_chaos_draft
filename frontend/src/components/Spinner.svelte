<script lang="ts">
  // A small loading spinner with its label (also read out by screen readers).
  let { label = 'Loading…' }: { label?: string } = $props();
</script>

<p class="spinner" role="status"><span class="spinner__ring" aria-hidden="true"></span>{label}</p>

<style>
  .spinner { display: inline-flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; color: var(--color-text-muted); margin: 0.5rem 0; }
  .spinner__ring {
    width: 1rem; height: 1rem; border-radius: 50%;
    border: 2px solid var(--color-border); border-top-color: var(--color-accent);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spinner__ring { animation-duration: 2.4s; } }
</style>

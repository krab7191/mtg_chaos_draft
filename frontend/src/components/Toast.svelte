<script lang="ts">
  import { toast } from '../lib/toast.svelte';
</script>

{#if toast.visible}
  <div
    class="toast"
    class:toast--error={toast.type === 'error'}
    class:toast--success={toast.type === 'success'}
  >
    <span class="toast__msg">{toast.msg}</span>
    {#if toast.mode === 'confirm'}
      <div class="toast__actions">
        <button class="toast__confirm" onclick={() => { toast.dismiss(); toast.onConfirm?.(); }}>Remove</button>
        <button class="toast__cancel"  onclick={() => { toast.dismiss(); toast.onCancel?.();  }}>Cancel</button>
      </div>
    {/if}
  </div>
{/if}

<style>
  .toast {
    position: fixed;
    bottom: 1.5rem;
    left: 50%;
    transform: translateX(-50%);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.6rem 0.75rem 0.6rem 1rem;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.88rem;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
    z-index: 500;
    max-width: min(420px, calc(100vw - 2rem));
    animation: toast-in 0.18s ease;
  }

  .toast--error   { border-color: var(--color-danger); }
  .toast--success { border-color: var(--color-success); }

  .toast__msg {
    color: var(--color-text);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .toast__actions {
    display: flex;
    gap: 0.4rem;
    flex-shrink: 0;
  }

  .toast__confirm,
  .toast__cancel {
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.18rem 0.55rem;
    font-size: 0.82rem;
    cursor: pointer;
    white-space: nowrap;
  }

  .toast__confirm {
    color: var(--color-danger);
  }
  .toast__confirm:hover { border-color: var(--color-danger); }

  .toast__cancel {
    color: var(--color-text-muted);
  }
  .toast__cancel:hover { border-color: var(--color-text-muted); color: var(--color-text); }

  @keyframes toast-in {
    from { opacity: 0; transform: translateX(-50%) translateY(6px); }
    to   { opacity: 1; transform: translateX(-50%) translateY(0); }
  }
</style>

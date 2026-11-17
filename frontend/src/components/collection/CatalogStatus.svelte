<script lang="ts">
  import { onMount } from 'svelte';
  import { getJSON, toastNetworkError, toastResponseError } from '../../lib/api';
  import { toast } from '../../lib/toast.svelte';

  interface Status {
    available: boolean;
    fetchedAt: string | null;
    source: 'live' | 'upload' | '';
    products: number;
  }

  let { isAdmin = false }: { isAdmin?: boolean } = $props();

  let status = $state<Status | null>(null);
  let uploading = $state(false);
  let fileInput = $state<HTMLInputElement | null>(null);

  // A live catalog is refreshed hourly, so anything older means MTGStocks
  // has been refusing the server.
  const STALE_MS = 3 * 60 * 60 * 1000;

  onMount(async () => {
    status = await getJSON<Status>('/api/catalog', "Couldn't check the price catalog");
  });

  function age(iso: string): string {
    const mins = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
    if (mins < 60) return `${Math.max(mins, 1)} min ago`;
    const hours = Math.round(mins / 60);
    if (hours < 48) return `${hours} h ago`;
    return `${Math.round(hours / 24)} days ago`;
  }

  const stale = $derived(
    !!status?.fetchedAt && (status.source === 'upload' || Date.now() - new Date(status.fetchedAt).getTime() > STALE_MS),
  );

  async function upload(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    uploading = true;
    try {
      const res = await fetch('/api/catalog', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: file,
      });
      if (!res.ok) return await toastResponseError('Catalog upload failed', res);
      status = await res.json();
      toast.success(`Catalog updated: ${status?.products} products, collection prices refreshed`);
    } catch {
      toastNetworkError('Catalog upload failed');
    } finally {
      uploading = false;
      if (fileInput) fileInput.value = '';
    }
  }
</script>

{#if status}
  <div class="catalog" class:catalog--warn={!status.available || stale}>
    <span class="catalog__text">
      {#if !status.available}
        No MTGStocks price catalog — search won't work until {isAdmin ? 'you upload' : 'an admin uploads'} one.
      {:else if status.source === 'upload'}
        Prices from an uploaded snapshot, {age(status.fetchedAt!)} ({status.products} products).
      {:else if stale}
        MTGStocks unreachable — using prices from {age(status.fetchedAt!)}.
      {:else}
        Prices live from MTGStocks, updated {age(status.fetchedAt!)}.
      {/if}
    </span>
    {#if isAdmin}
      <label class="catalog__upload" title="JSON saved from api.mtgstocks.com/sealed">
        {uploading ? 'Uploading…' : 'Upload snapshot'}
        <input bind:this={fileInput} type="file" accept="application/json,.json" onchange={upload} disabled={uploading} hidden />
      </label>
    {/if}
  </div>
{/if}

<style>
  .catalog {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem 0.75rem;
    margin-bottom: 1rem;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.85rem;
    color: var(--color-text-muted);
  }
  .catalog--warn { border-color: var(--color-danger); color: var(--color-text); }
  .catalog__text { flex: 1; min-width: 12rem; }
  .catalog__upload {
    cursor: pointer;
    white-space: nowrap;
    padding: 0.25rem 0.6rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
  }
  .catalog__upload:hover { border-color: var(--color-text-muted); }
</style>

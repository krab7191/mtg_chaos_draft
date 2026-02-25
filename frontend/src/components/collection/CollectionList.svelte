<script lang="ts">
  import { untrack } from 'svelte';
  import CollectionCard from './CollectionCard.svelte';
  import { computeSortedSets, type Pack } from '../../lib/collection';
  import { toast } from '../../lib/toast.svelte';

  let { packs: initialPacks }: { packs: Pack[] } = $props();

  // ── State ──────────────────────────────────────────────────
  let packs      = $state<Pack[]>(untrack(() => initialPacks.map(p => ({ ...p }))));
  let sortKey    = $state<'name' | 'price' | 'qty'>('name');
  let sortDir    = $state<'asc' | 'desc'>('asc');

  // ── Derived ────────────────────────────────────────────────
  const totalCount = $derived(packs.reduce((s, p) => s + p.quantity, 0));
  const totalValue = $derived(packs.reduce((s, p) => s + (p.marketPrice ?? 0) * p.quantity, 0));

  const sortedSets = $derived(computeSortedSets(packs, sortKey, sortDir));

  // ── Sort ───────────────────────────────────────────────────
  function onSortClick(key: typeof sortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = key === 'name' ? 'asc' : 'desc';  // name asc, price/qty desc by default
    }
  }

  function sortLabel(key: typeof sortKey, label: string) {
    if (sortKey !== key) return label;
    return label + (sortDir === 'asc' ? ' ↑' : ' ↓');
  }

  // ── Qty update ─────────────────────────────────────────────
  async function onQtyChange(id: number, delta: number) {
    const pack = packs.find(p => p.id === id);
    if (!pack) return;
    const prev = pack.quantity;
    const next = Math.max(0, prev + delta);
    pack.quantity = next;
    const res = await fetch(`/api/collection/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ quantity: next }),
    });
    if (!res.ok) {
      pack.quantity = prev;
      toast.show(`Failed to update quantity: ${res.status} ${res.statusText}`, 'error');
    }
  }

  // ── Delete ─────────────────────────────────────────────────
  function onDelete(id: number, label: string) {
    toast.confirm(`Remove "${label}"?`, async () => {
      const res = await fetch(`/api/collection/${id}`, { method: 'DELETE' });
      if (!res.ok) {
        toast.show(`Failed to delete: ${res.status} ${res.statusText}`, 'error');
        return;
      }
      packs = packs.filter(p => p.id !== id);
    });
  }
</script>

<!-- ── Header ────────────────────────────────────────────── -->
<div class="collection__header">
  <h2 class="collection__heading">In your collection</h2>
  {#if totalCount > 0}
    <span class="collection__value"><strong>{totalCount}</strong> packs</span>
  {/if}
  {#if totalValue > 0}
    <span class="collection__value"><strong>${totalValue.toFixed(2)}</strong></span>
  {/if}
</div>

<!-- ── Sort ──────────────────────────────────────────────── -->
<div class="collection__sort">
  <span class="collection__sort-label">Sort:</span>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'name'}
    onclick={() => onSortClick('name')}
  >{sortLabel('name', 'Set')}</button>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'qty'}
    onclick={() => onSortClick('qty')}
  >{sortLabel('qty', 'Qty')}</button>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'price'}
    onclick={() => onSortClick('price')}
  >{sortLabel('price', 'Price')}</button>
</div>

<p class="collection__disclaimer">Prices update automatically once a day.</p>
{#if packs.some(p => p.cardsPerPack < 12)}
  <p class="collection__footnote">* Non-standard pack size; multiples needed.</p>
{/if}

<!-- ── List ──────────────────────────────────────────────── -->
{#if packs.length === 0}
  <p class="collection__empty">Nothing yet — search above to add some packs.</p>
{:else}
  <div class="pack-list">
    {#each sortedSets as [groupKey, setPacks] (groupKey)}
      <CollectionCard
        setName={setPacks[0].setName}
        packs={setPacks}
        {onQtyChange}
        {onDelete}
      />
    {/each}
  </div>
{/if}

<style>
  .collection__header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }

  .collection__heading {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-text-muted);
    margin-bottom: 0;
  }

  .collection__value {
    font-size: 0.85rem;
    color: var(--color-text-muted);
  }
  .collection__value strong {
    color: var(--color-text);
    font-variant-numeric: tabular-nums;
  }

  .collection__sort {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.6rem;
  }

  .collection__sort-label {
    font-size: 0.78rem;
    color: var(--color-text-muted);
  }

  .collection__disclaimer {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    opacity: 0.6;
    margin-bottom: 0.75rem;
  }

  .collection__footnote {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    opacity: 0.7;
    margin-bottom: 0.75rem;
  }

  .collection__empty {
    color: var(--color-text-muted);
    padding: 2rem 0;
    text-align: center;
    font-size: 0.9rem;
  }

  .sort-btn {
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.2rem 0.55rem;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    cursor: pointer;
    transition: border-color 0.1s, color 0.1s;
  }
  .sort-btn:hover { border-color: var(--color-accent); color: var(--color-accent); }
  .sort-btn--active { border-color: var(--color-accent); color: var(--color-accent); }

  .pack-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
</style>

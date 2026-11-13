<script lang="ts" module>
  export interface PickerPack {
    id: number;
    setName: string;
    productType: string;
    mtgstocksId?: number | null;
  }

  /** A product from the MTGStocks search, not in the collection yet. */
  export interface CatalogProduct {
    mtgstocksId: number;
    name: string;
    setName: string;
    setCode: string;
    productType: string;
    marketPrice: number | null;
  }

  export type PackChoice =
    | { kind: 'pack'; pack: PickerPack }
    | { kind: 'new'; product: CatalogProduct };
</script>

<script lang="ts">
  // Type to find a pack: the collection first, then (optionally) MTGStocks for
  // packs that aren't in the collection yet.
  let { packs, value = $bindable(null), searchCatalog = false, describe = () => '', label = 'Pack' }: {
    packs: PickerPack[];
    value?: PackChoice | null;
    searchCatalog?: boolean;
    /** Extra text after a collection pack's name, e.g. "(12 in stock)". */
    describe?: (p: PickerPack) => string;
    label?: string;
  } = $props();

  const MAX_LOCAL = 8;
  let query    = $state('');
  let open     = $state(false);
  let catalog  = $state<CatalogProduct[]>([]);
  let searching = $state(false);
  let timer: ReturnType<typeof setTimeout> | null = null;
  let seq = 0;

  const nameOf = (p: { setName: string; productType: string }) => `${p.setName} — ${p.productType}`;

  const words = $derived(query.toLowerCase().split(/\s+/).filter(Boolean));
  const matches = $derived(
    words.length === 0
      ? packs.slice(0, MAX_LOCAL)
      : packs.filter(p => {
          const hay = nameOf(p).toLowerCase();
          return words.every(w => hay.includes(w));
        }).slice(0, MAX_LOCAL),
  );
  const known = $derived(new Set(packs.map(p => p.mtgstocksId).filter((x): x is number => x != null)));
  const newProducts = $derived(catalog.filter(c => !known.has(c.mtgstocksId)));

  // Clear the choice when it's cleared from the input (e.g. after saving).
  $effect(() => {
    if (value == null && !open) query = '';
  });

  function onInput(e: Event) {
    query = (e.target as HTMLInputElement).value;
    value = null;
    open = true;
    if (!searchCatalog) return;
    if (timer) clearTimeout(timer);
    const q = query.trim();
    if (q.length < 2) { catalog = []; searching = false; return; }
    searching = true;
    timer = setTimeout(() => search(q), 400);
  }

  async function search(q: string) {
    const mine = ++seq;
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(q)}`);
      const items = res.ok ? await res.json() : [];
      if (mine === seq) catalog = Array.isArray(items) ? items : [];
    } catch {
      if (mine === seq) catalog = [];
    } finally {
      if (mine === seq) searching = false;
    }
  }

  function choose(c: PackChoice) {
    if (timer) { clearTimeout(timer); timer = null; }
    seq++; // ignore a search still in flight
    searching = false;
    value = c;
    query = c.kind === 'pack' ? nameOf(c.pack) : nameOf(c.product);
    open = false;
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { open = false; return; }
    if (e.key !== 'Enter' || !open) return;
    e.preventDefault();
    if (matches.length) choose({ kind: 'pack', pack: matches[0] });
    else if (newProducts.length) choose({ kind: 'new', product: newProducts[0] });
  }
</script>

<div class="picker field field--wide">
  <span>{label}</span>
  <input
    type="search"
    role="combobox"
    aria-expanded={open}
    aria-autocomplete="list"
    autocomplete="off"
    placeholder={searchCatalog ? 'Search your collection or any set…' : 'Search…'}
    value={query}
    oninput={onInput}
    onfocus={() => (open = true)}
    onblur={() => setTimeout(() => (open = false), 150)}
    onkeydown={onKeydown}
  />
  {#if value?.kind === 'new'}
    <span class="picker__note">Not in your collection yet: it'll be added.</span>
  {/if}
  {#if open}
    <ul class="picker__list" role="listbox">
      {#each matches as p (p.id)}
        <li role="option" aria-selected={value?.kind === 'pack' && value.pack.id === p.id}>
          <button type="button" class="picker__item" onmousedown={(e) => e.preventDefault()}
            onclick={() => choose({ kind: 'pack', pack: p })}>
            {nameOf(p)}<span class="picker__meta">{describe(p)}</span>
          </button>
        </li>
      {/each}
      {#if searchCatalog && query.trim().length >= 2}
        <li class="picker__heading">Not in your collection</li>
        {#each newProducts as c (c.mtgstocksId)}
          <li role="option" aria-selected={value?.kind === 'new' && value.product.mtgstocksId === c.mtgstocksId}>
            <button type="button" class="picker__item" onmousedown={(e) => e.preventDefault()}
              onclick={() => choose({ kind: 'new', product: c })}>
              {nameOf(c)}<span class="picker__meta">{c.marketPrice != null ? ` $${c.marketPrice.toFixed(2)}` : ''}</span>
            </button>
          </li>
        {:else}
          <li class="picker__empty">{searching ? 'Searching…' : 'No other matches'}</li>
        {/each}
      {:else if matches.length === 0}
        <li class="picker__empty">No matches</li>
      {/if}
    </ul>
  {/if}
</div>

<style>
  .picker { position: relative; }
  .picker input { width: 100%; box-sizing: border-box; }
  .picker__note { font-size: 0.7rem; color: var(--color-accent); }
  .picker__list {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    z-index: 50;
    margin: 0.2rem 0 0;
    padding: 0.25rem 0;
    list-style: none;
    max-height: 18rem;
    overflow-y: auto;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  }
  .picker__item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: var(--color-text);
    font-size: 0.85rem;
    padding: 0.35rem 0.7rem;
    cursor: pointer;
  }
  .picker__item:hover { background: var(--color-surface-2, rgba(255, 255, 255, 0.05)); }
  .picker__meta { color: var(--color-text-muted); font-size: 0.78rem; margin-left: 0.35rem; }
  .picker__heading {
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
    padding: 0.45rem 0.7rem 0.2rem;
    border-top: 1px solid var(--color-border);
    margin-top: 0.25rem;
  }
  .picker__empty { font-size: 0.8rem; color: var(--color-text-muted); padding: 0.35rem 0.7rem; }
</style>

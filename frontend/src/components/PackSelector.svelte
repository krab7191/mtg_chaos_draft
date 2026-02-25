<script lang="ts">
  import { toast } from '../lib/toast.svelte';

  interface Pack {
    id: number;
    name: string;
    setName: string;
    productType: string;
    quantity: number;
    cardsPerPack: number;
    marketPrice?: number | null;
  }

  interface HistoryEntry {
    packId: number | null;
    setName: string;
    productType: string;
    marketPrice: number | null;
    price: string;
  }

  let { packs, settings }: {
    packs: Pack[];
    settings: {
      priceFloor:  number;
      priceCap:    number;
      quantityCap: number;
      packWeights: Record<string, number>;
    };
  } = $props();

  const packWeights = $derived(settings.packWeights ?? {});

  const HISTORY_KEY = 'draft-history';
  const MAX_HISTORY = 12;

  // ── Initialize from localStorage (SSR-safe) ─────────────────
  function readHistory(): HistoryEntry[] {
    if (typeof localStorage === 'undefined') return [];
    try { return JSON.parse(localStorage.getItem(HISTORY_KEY) ?? '[]'); } catch { return []; }
  }
  const _init = readHistory();

  // ── State ───────────────────────────────────────────────────
  let checked         = $state(new Set(packs.map(p => String(p.id))));
  let sortKey         = $state<'name' | 'price'>('name');
  let sortDir         = $state<'asc' | 'desc'>('asc');
  let result          = $state<{ setName: string; productType: string } | null>(null);
  let history         = $state<HistoryEntry[]>(_init);
  let recordDraft     = $state(true);
  let hideDeselected  = $state(true);
  let pickedCounts    = $state<Record<string, number>>({});

  // ── Persist history whenever it changes ─────────────────────
  $effect(() => {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
    }
  });

  // ── Toggle recording ────────────────────────────────────────
  function setRecordDraft(on: boolean) {
    recordDraft = on;
    if (!on) {
      for (const key in pickedCounts) delete pickedCounts[key];
    }
  }

  // ── Derived ─────────────────────────────────────────────────
  const sortedPacks = $derived.by(() => {
    const copy = hideDeselected ? packs.filter(p => checked.has(String(p.id))) : [...packs];
    if (sortKey === 'price') {
      copy.sort((a, b) => {
        const pa = a.marketPrice ?? 0;
        const pb = b.marketPrice ?? 0;
        return sortDir === 'asc' ? pa - pb : pb - pa;
      });
    } else {
      copy.sort((a, b) => {
        const cmp = a.setName.localeCompare(b.setName) || a.productType.localeCompare(b.productType);
        return sortDir === 'asc' ? cmp : -cmp;
      });
    }
    return copy;
  });

  const checkedPacks = $derived(packs.filter(p => checked.has(String(p.id))));

  function packsPerSlot(p: Pack): number {
    return Math.ceil(15 / Math.max(1, p.cardsPerPack));
  }

  function effectiveQty(p: Pack): number {
    const slots = packsPerSlot(p);
    return Math.max(0, Math.floor(p.quantity / slots) - (pickedCounts[String(p.id)] ?? 0));
  }

  function computeWeights(activePacks: Pack[]): number[] {
    const floor  = settings.priceFloor  > 0 ? settings.priceFloor  : null;
    const cap    = settings.priceCap    > 0 ? settings.priceCap    : null;
    const qtyCap = settings.quantityCap > 0 ? settings.quantityCap : null;

    const capPrice = (p: number) => {
      if (floor && p < floor) p = floor;
      if (cap   && p > cap)   p = cap;
      return p;
    };

    const rawPrices = activePacks.map(p => {
      const mp = p.marketPrice;
      return mp && mp > 0 ? capPrice(mp) : null;
    });
    const validPrices = rawPrices.filter((p): p is number => p !== null);
    const avgPrice = validPrices.length ? validPrices.reduce((a, b) => a + b, 0) / validPrices.length : 1;

    return activePacks.map((p, i) => {
      const qty = qtyCap ? Math.min(effectiveQty(p), qtyCap) : effectiveQty(p);
      if (qty === 0) return 0;
      const price = rawPrices[i] ?? avgPrice;
      const mult = packWeights[String(p.id)] ?? 0;
      return (qty / price) * Math.max(0, 1 + mult);
    });
  }

  function weightedRandom(activePacks: Pack[], weights: number[]): Pack {
    const total = weights.reduce((a, b) => a + b, 0);
    let r = Math.random() * total;
    for (let i = 0; i < activePacks.length; i++) {
      r -= weights[i];
      if (r <= 0) return activePacks[i];
    }
    return activePacks[activePacks.length - 1];
  }

  const odds = $derived.by(() => {
    const active = checkedPacks;
    const weights = computeWeights(active);
    const total = weights.reduce((a, b) => a + b, 0);
    const result: Record<string, number> = {};
    active.forEach((p, i) => {
      result[String(p.id)] = total > 0 ? (weights[i] / total) * 100 : 0;
    });
    return result;
  });

  const allChecked  = $derived(checked.size === packs.length);
  const noneChecked = $derived(checked.size === 0);

  // ── Helpers ─────────────────────────────────────────────────
  function toggle(id: string) {
    const next = new Set(checked);
    if (next.has(id)) next.delete(id); else next.add(id);
    checked = next;
  }

  function selectAll()   { checked = new Set(packs.map(p => String(p.id))); }
  function deselectAll() { checked = new Set(); }

  function onSortClick(key: typeof sortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = key === 'name' ? 'asc' : 'desc';
    }
  }

  function sortLabel(key: typeof sortKey, label: string) {
    if (sortKey !== key) return label;
    return label + (sortDir === 'asc' ? ' ↑' : ' ↓');
  }

  // ── Draft save ──────────────────────────────────────────────
  function saveDraft(entries: HistoryEntry[]) {
    fetch('/api/drafts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        picks: entries.map(e => ({
          packId: e.packId ?? null,
          setName: e.setName,
          productType: e.productType,
          marketPrice: e.marketPrice ?? null,
        })),
      }),
    }).then(res => {
      if (res.ok) {
        toast.show('Draft saved', 'success');
      } else {
        toast.show('Failed to save draft', 'error');
      }
    }).catch(() => {
      toast.show('Failed to save draft', 'error');
    });
  }

  // ── Pick ────────────────────────────────────────────────────
  let toastTimer: ReturnType<typeof setTimeout> | null = null;

  function dismissResult() {
    result = null;
    if (toastTimer) { clearTimeout(toastTimer); toastTimer = null; }
  }

  function scheduleToastDismiss() {
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { result = null; toastTimer = null; }, 3000);
  }

  function pick() {
    const active = checkedPacks;
    if (active.length === 0) return;

    const weights = computeWeights(active);
    const selectedPack = weightedRandom(active, weights);

    result = { setName: selectedPack.setName, productType: selectedPack.productType };
    scheduleToastDismiss();

    if (recordDraft && history.length < MAX_HISTORY) {
      const price = selectedPack.marketPrice != null ? `$${selectedPack.marketPrice.toFixed(2)}` : '—';
      const newHistory = [...history, {
        packId: selectedPack.id,
        setName: selectedPack.setName,
        productType: selectedPack.productType,
        marketPrice: selectedPack.marketPrice ?? null,
        price,
      }];
      history = newHistory;

      const id = String(selectedPack.id);
      pickedCounts[id] = (pickedCounts[id] ?? 0) + 1;
      if (effectiveQty(selectedPack) === 0) {
        const next = new Set(checked);
        next.delete(id);
        checked = next;
      }

      if (newHistory.length >= MAX_HISTORY) {
        saveDraft(newHistory);
      }
    }
  }
</script>

{#if packs.length === 0}
  <p class="empty">The collection is empty. Ask your admin to add some packs!</p>
{:else}
  <!-- Floating count pill -->
  <div class="count-pill">{checked.size} selected</div>

  <!-- Controls -->
  <div class="controls">
    <button class="btn btn--secondary" onclick={selectAll} disabled={allChecked}>Select all</button>
    <button class="btn btn--secondary" onclick={deselectAll} disabled={noneChecked}>Deselect all</button>
    <label class="toggle-label">
      <input type="checkbox" bind:checked={hideDeselected} />
      Hide deselected
    </label>
  </div>

  <!-- Sort bar -->
  <div class="pack-sort">
    <span class="pack-sort__label">Sort:</span>
    <button
      class="sort-btn"
      class:sort-btn--active={sortKey === 'name'}
      onclick={() => onSortClick('name')}
    >{sortLabel('name', 'Name')}</button>
    <button
      class="sort-btn"
      class:sort-btn--active={sortKey === 'price'}
      onclick={() => onSortClick('price')}
    >{sortLabel('price', 'Price')}</button>
  </div>

  <!-- Pack list -->
  <ul class="pack-list">
    {#each sortedPacks as pack (pack.id)}
      {@const id = String(pack.id)}
      {@const isChecked = checked.has(id)}
      {@const packOdds = isChecked ? (odds[id] ?? 0) : null}
      <li class="pack-item">
        <label class="pack-item__label" class:pack-item__label--checked={isChecked} class:pack-item__label--depleted={effectiveQty(pack) === 0}>
          <input
            type="checkbox"
            class="pack-checkbox"
            checked={isChecked}
            onchange={() => toggle(id)}
          />
          <span class="pack-item__info">
            <span class="pack-item__name">{pack.setName}</span>
            <span class="pack-item__meta">{pack.productType} ({effectiveQty(pack)}{pack.cardsPerPack < 12 ? ' *' : ''})</span>
          </span>
          <span class="pack-item__price">
            {pack.marketPrice != null ? `$${pack.marketPrice.toFixed(2)}` : '—'}
          </span>
          <span class="pack-item__odds">
            {packOdds != null ? `${packOdds.toFixed(1)}%` : '—'}
          </span>
        </label>
      </li>
    {/each}
  </ul>

  <!-- Non-standard pack size footnote -->
  {#if checkedPacks.some(p => p.cardsPerPack < 12)}
    <ul class="pack-footnote">
      {#each checkedPacks.filter(p => p.cardsPerPack < 12) as p}
        {@const slots = packsPerSlot(p)}
        <li>⁎ {p.setName} {p.productType}: {p.cardsPerPack} cards/pack ({slots}× per slot)</li>
      {/each}
    </ul>
  {/if}

  <!-- Pick row -->
  <div class="pick-row">
    <button
      class="btn btn--primary btn--large"
      disabled={noneChecked || history.length >= MAX_HISTORY}
      onclick={pick}
    >🎲 Pick a Pack</button>
    <label class="history-toggle">
      <input type="checkbox" checked={recordDraft} disabled={history.length >= MAX_HISTORY} onchange={(e) => setRecordDraft((e.target as HTMLInputElement).checked)} />
      Record draft
    </label>
    {#if history.length >= MAX_HISTORY}
      <span class="draft-full">Draft complete ({MAX_HISTORY}/{MAX_HISTORY})</span>
    {/if}
  </div>

  <!-- Result toast -->
  {#if result}
    <div class="toast" role="status" onclick={dismissResult}>
      <span class="toast__text">{result.setName}{result.productType ? ` · ${result.productType}` : ''}</span>
    </div>
  {/if}

  <!-- History -->
  {#if recordDraft && history.length > 0}
    <div class="history">
      <div class="history__header">
        <span class="history__title">Draft history</span>
        <button class="history__clear" onclick={() => { history = []; pickedCounts = {}; }}>Clear</button>
      </div>
      <ol class="history__list">
        {#each history as entry, i}
          <li class="history__item">
            <span class="history__num">{i + 1}.</span>
            <span class="history__pack-info">
              <span class="history__pack-name">{entry.setName}</span>
              <span class="history__pack-set">{entry.productType}</span>
            </span>
            <span class="history__pack-price">{entry.price}</span>
          </li>
        {/each}
      </ol>
    </div>
  {/if}
{/if}

<style>
  .empty {
    color: var(--color-text-muted);
    text-align: center;
    padding: 3rem 0;
  }

  /* ── Controls ──────────────────────────────────────────────── */
  .controls {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-bottom: 0.9rem;
  }

  .count-pill {
    position: fixed;
    top: 5.25rem;
    right: 1rem;
    background: var(--color-surface);
    border: 1px solid var(--color-accent);
    border-radius: 8px;
    padding: 0.25rem 0.75rem;
    font-size: 0.82rem;
    font-weight: 500;
    color: var(--color-accent);
    z-index: 100;
    pointer-events: none;
  }

  /* ── Sort bar ──────────────────────────────────────────────── */
  .pack-sort {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.75rem;
  }

  .pack-sort__label {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    margin-right: 0.2rem;
  }

  .sort-btn {
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text-muted);
    font-size: 0.78rem;
    padding: 0.2rem 0.55rem;
    cursor: pointer;
    transition: border-color 0.1s, color 0.1s;
  }
  .sort-btn:hover { border-color: var(--color-accent); color: var(--color-text); }
  .sort-btn--active { border-color: var(--color-accent); color: var(--color-accent); }

  /* ── Pack list ─────────────────────────────────────────────── */
  .pack-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-bottom: 1.25rem;
    padding: 0;
  }

  @media (min-width: 480px) {
    .pack-list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
      gap: 0.5rem;
    }
  }

  .pack-item {
    min-width: 0;
  }

  .pack-item__label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.65rem 0.8rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: border-color 0.1s;
    min-height: 2.8rem;
    min-width: 0;
    overflow: hidden;
  }
  .pack-item__label:hover { border-color: var(--color-accent); }
  .pack-item__label--depleted { opacity: 0.4; }

  .pack-item__info {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .pack-item__name {
    font-weight: 500;
    font-size: 0.88rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .pack-item__meta {
    color: var(--color-text-muted);
    font-size: 0.73rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .pack-item__price {
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .pack-item__odds {
    color: var(--color-accent);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .pack-checkbox { accent-color: var(--color-accent); flex-shrink: 0; }

  /* ── Footnote ──────────────────────────────────────────────── */
  .pack-footnote {
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
    font-size: 0.73rem;
    color: var(--color-text-muted);
    opacity: 0.8;
  }

  /* ── Pick row ──────────────────────────────────────────────── */
  .pick-row {
    position: fixed;
    bottom: 1.5rem;
    left: 1.5rem;
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    z-index: 100;
  }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.88rem;
    color: var(--color-text-muted);
    cursor: pointer;
    user-select: none;
  }
  .toggle-label input { accent-color: var(--color-accent); }

  .history-toggle {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.88rem;
    color: var(--color-text-muted);
    cursor: pointer;
    user-select: none;
  }
  .history-toggle input { accent-color: var(--color-accent); }

  .draft-full {
    font-size: 0.78rem;
    color: var(--color-accent);
    opacity: 0.8;
  }

  /* ── Toast ─────────────────────────────────────────────────── */
  .toast {
    position: fixed;
    top: 1rem;
    left: 1rem;
    right: 1rem;
    background: var(--color-surface);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius);
    padding: 0.85rem 1.75rem;
    font-size: 1.05rem;
    font-weight: 500;
    box-sizing: border-box;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.35);
    z-index: 250;
    cursor: pointer;
    animation: slideDown 0.2s ease;
  }

  .toast__text {
    color: var(--color-text);
    display: block;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  @keyframes slideDown {
    from { transform: translateX(-50%) translateY(-0.75rem); opacity: 0; }
    to   { transform: translateX(-50%) translateY(0);        opacity: 1; }
  }

  /* ── History ───────────────────────────────────────────────── */
  .history {
    margin-top: 1.5rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .history__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.55rem 0.85rem;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
  }

  .history__title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--color-text-muted);
  }

  .history__clear {
    background: none;
    border: none;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    cursor: pointer;
    padding: 0;
  }
  .history__clear:hover { color: var(--color-danger); }

  .history__list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .history__item {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    padding: 0.5rem 0.85rem;
    border-bottom: 1px solid var(--color-border);
    font-size: 0.88rem;
  }
  .history__item:last-child { border-bottom: none; }

  .history__num {
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-variant-numeric: tabular-nums;
    min-width: 1.2rem;
  }

  .history__pack-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .history__pack-name {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .history__pack-set {
    color: var(--color-text-muted);
    font-size: 0.75rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .history__pack-price {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  /* ── Buttons ───────────────────────────────────────────────── */
  .btn {
    border: none;
    border-radius: var(--radius);
    padding: 0.5rem 1rem;
    font-size: 0.9rem;
    font-weight: 500;
    cursor: pointer;
    transition: opacity 0.15s;
  }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn:hover:not(:disabled) { opacity: 0.85; }

  .btn--primary { background: var(--color-accent); color: white; }
  .btn--secondary {
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
  }
  .btn--large {
    padding: 0.85rem 1.5rem;
    font-size: 1rem;
  }

  @media (min-width: 480px) {
    .btn--large {
      padding: 0.75rem 2rem;
      font-size: 1.1rem;
    }
  }
</style>

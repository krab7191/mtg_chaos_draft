<script lang="ts">
  interface Pack {
    id: number;
    name: string;
    setName: string;
    productType: string;
    quantity: number;
    marketPrice?: number | null;
  }

  interface HistoryEntry {
    setName: string;
    productType: string;
    price: string;
  }

  let { packs, packWeights }: {
    packs: Pack[];
    packWeights: Record<string, number>;
  } = $props();

  const HISTORY_KEY = 'draft-history';
  const MAX_HISTORY = 12;

  // ── State ───────────────────────────────────────────────────
  let checked      = $state(new Set(packs.map(p => String(p.id))));
  let sortKey      = $state<'name' | 'price'>('name');
  let sortDir      = $state<'asc' | 'desc'>('asc');
  let picking      = $state(false);
  let result       = $state<{ setName: string; productType: string } | null>(null);
  let history      = $state<HistoryEntry[]>([]);
  let showHistory  = $state(false);
  let recordDraft  = $state(false);

  // ── Load history on mount ───────────────────────────────────
  $effect(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(HISTORY_KEY) ?? '[]') as HistoryEntry[];
      history = stored;
      recordDraft = stored.length > 0 && stored.length < MAX_HISTORY;
      showHistory = recordDraft && stored.length > 0;
    } catch { history = []; }
  });

  // ── Save history when it changes ────────────────────────────
  $effect(() => {
    // Access history to subscribe; untrack to avoid loops when we set it above
    const h = history;
    localStorage.setItem(HISTORY_KEY, JSON.stringify(h));
  });

  // ── Derived ─────────────────────────────────────────────────
  const sortedPacks = $derived.by(() => {
    const copy = [...packs];
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

  const odds = $derived.by(() => {
    const active = checkedPacks;
    const prices = active
      .map(p => (p.marketPrice && p.marketPrice > 0) ? p.marketPrice : null)
      .filter((p): p is number => p !== null);
    const avgPrice = prices.length ? prices.reduce((a, b) => a + b, 0) / prices.length : 1;

    const weights = active.map(p => {
      if (p.quantity === 0) return 0;
      const price = (p.marketPrice && p.marketPrice > 0) ? p.marketPrice : avgPrice;
      const mult = packWeights[String(p.id)] ?? 0;
      return (p.quantity / price) * Math.max(0, 1 + mult);
    });
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

  async function pick() {
    const ids = checkedPacks.map(p => p.id);
    if (ids.length === 0) return;
    picking = true;
    const res = await fetch('/api/select', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ packIds: ids }),
    });
    picking = false;
    if (!res.ok) return;
    const { selectedPack } = await res.json();
    result = { setName: selectedPack.setName, productType: selectedPack.productType };

    if (recordDraft && history.length < MAX_HISTORY) {
      const price = selectedPack.marketPrice != null ? `$${selectedPack.marketPrice.toFixed(2)}` : '—';
      history = [...history, { setName: selectedPack.setName, productType: selectedPack.productType, price }];
      if (history.length >= MAX_HISTORY) recordDraft = false;
    }

    showHistory = recordDraft && history.length > 0;
    scheduleToastDismiss();
  }
</script>

{#if packs.length === 0}
  <p class="empty">The collection is empty. Ask your admin to add some packs!</p>
{:else}
  <!-- Controls -->
  <div class="controls">
    <button class="btn btn--secondary" onclick={selectAll} disabled={allChecked}>Select all</button>
    <button class="btn btn--secondary" onclick={deselectAll} disabled={noneChecked}>Deselect all</button>
    <span class="selected-count">{checked.size} selected</span>
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
        <label class="pack-item__label" class:pack-item__label--checked={isChecked}>
          <input
            type="checkbox"
            class="pack-checkbox"
            checked={isChecked}
            onchange={() => toggle(id)}
          />
          <span class="pack-item__info">
            <span class="pack-item__name">{pack.setName}</span>
            <span class="pack-item__meta">{pack.productType}</span>
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

  <!-- Pick row -->
  <div class="pick-row">
    <button
      class="btn btn--primary btn--large"
      disabled={picking || noneChecked}
      onclick={pick}
    >{picking ? '🎲 Picking...' : '🎲 Pick a Pack'}</button>
    <label class="history-toggle">
      <input type="checkbox" bind:checked={recordDraft} onchange={() => { showHistory = recordDraft && history.length > 0; }} />
      Record draft
    </label>
    {#if history.length > 0 && recordDraft}
      <button class="btn btn--secondary" onclick={() => { showHistory = !showHistory; }}>
        {showHistory ? 'Hide history' : 'Show history'}
      </button>
    {/if}
  </div>

  <!-- Result toast -->
  {#if result}
    <div class="toast" role="status" onclick={dismissResult}>
      <span class="toast__text">{result.setName}{result.productType ? ` · ${result.productType}` : ''}</span>
    </div>
  {/if}

  <!-- History -->
  {#if showHistory && history.length > 0}
    <div class="history">
      <div class="history__header">
        <span class="history__title">Draft history</span>
        <button class="history__clear" onclick={() => { history = []; showHistory = false; }}>Clear</button>
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

  .selected-count {
    color: var(--color-text-muted);
    font-size: 0.82rem;
    margin-left: auto;
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

  /* ── Pick row ──────────────────────────────────────────────── */
  .pick-row {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }

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

  /* ── Toast ─────────────────────────────────────────────────── */
  .toast {
    position: fixed;
    top: 3.75rem;
    left: 50%;
    transform: translateX(-50%);
    background: var(--color-surface);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius);
    padding: 0.85rem 1.75rem;
    font-size: 1.05rem;
    font-weight: 500;
    white-space: nowrap;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.35);
    z-index: 50;
    cursor: pointer;
    animation: slideDown 0.2s ease;
  }

  .toast__text { color: var(--color-text); }

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

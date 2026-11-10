<script lang="ts">
  import { untrack } from 'svelte';
  import { toast } from '../lib/toast.svelte';
  import { groupPicks } from '../lib/draft';

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
    key?: string;
    packId: number | null;
    setName: string;
    productType: string;
    /** Price of one physical pack at pick time (live MTGStocks price when available). */
    unitPrice?: number | null;
    /** Physical packs consumed by this slot (small packs fill a slot with 2–3). */
    packCount?: number;
    /** Total cost of the slot: unitPrice × packCount. */
    marketPrice: number | null;
    /** True while the live price is being fetched. */
    pricing?: boolean;
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

  const HISTORY_KEY  = 'draft-history';
  const POD_SIZE_KEY = 'draft-pod-size';
  const PREFS_KEY    = 'pack-selector-prefs';
  const PACKS_PER_PLAYER = 3;
  const MIN_POD_SIZE = 2;
  const MAX_POD_SIZE = 16;
  const DEFAULT_POD_SIZE = 4;

  // ── Initialize from localStorage (SSR-safe) ─────────────────
  function readHistory(): HistoryEntry[] {
    try {
      const raw: HistoryEntry[] = JSON.parse(localStorage.getItem(HISTORY_KEY) ?? '[]');
      // Entries restored after a reload can't still be fetching.
      return raw.map(e => ({ ...e, pricing: false }));
    } catch { return []; }
  }
  function readPodSize(): number {
    try {
      const n = Number(localStorage.getItem(POD_SIZE_KEY));
      return Number.isInteger(n) && n >= MIN_POD_SIZE && n <= MAX_POD_SIZE ? n : DEFAULT_POD_SIZE;
    } catch { return DEFAULT_POD_SIZE; }
  }
  interface Prefs {
    unchecked?: string[];
    sortKey?: string;
    sortDir?: string;
    hideDeselected?: boolean;
    recordDraft?: boolean;
    draftSaved?: boolean;
  }
  function readPrefs(): Prefs {
    try {
      const p = JSON.parse(localStorage.getItem(PREFS_KEY) ?? '{}');
      return p && typeof p === 'object' ? p : {};
    } catch { return {}; }
  }
  const _init  = readHistory();
  const _prefs = readPrefs();
  // Remember what was UNchecked, so packs added to the collection later start selected.
  const _unchecked = new Set(Array.isArray(_prefs.unchecked) ? _prefs.unchecked.map(String) : []);

  // ── State ───────────────────────────────────────────────────
  // Every pack starts selected except the ones unchecked before a reload.
  let checked         = $state(untrack(() => new Set(packs.map(p => String(p.id)).filter(id => !_unchecked.has(id)))));
  type SortKey = 'name' | 'qty' | 'price';
  let sortKey         = $state<SortKey>(
    _prefs.sortKey === 'price' || _prefs.sortKey === 'qty' ? _prefs.sortKey : 'name',
  );
  let sortDir         = $state<'asc' | 'desc'>(_prefs.sortDir === 'desc' ? 'desc' : 'asc');
  let result          = $state<{ setName: string; productType: string } | null>(null);
  let history         = $state<HistoryEntry[]>(_init);
  let recordDraft     = $state(_prefs.recordDraft !== false);
  let hideDeselected  = $state(_prefs.hideDeselected !== false);
  let podSize         = $state(readPodSize());
  let livePrices      = $state<Record<string, number>>({});
  let draftSaved      = $state(_prefs.draftSaved === true && _init.length > 0);
  let historyEl       = $state<HTMLElement | null>(null);

  // Picks per pack, worked out from the (persisted) history so stock stays
  // correct after a reload. Unrecorded picks don't use up stock.
  const pickedCounts = $derived.by(() => {
    const counts: Record<string, number> = {};
    if (!recordDraft) return counts;
    for (const e of history) {
      if (e.packId != null) counts[String(e.packId)] = (counts[String(e.packId)] ?? 0) + 1;
    }
    return counts;
  });

  const maxPicks   = $derived(podSize * PACKS_PER_PLAYER);
  const minPodSize = $derived(Math.max(MIN_POD_SIZE, Math.ceil(history.length / PACKS_PER_PLAYER)));
  const draftFull  = $derived(history.length >= maxPicks);


  const draftTotal   = $derived(history.reduce((sum, e) => sum + (e.marketPrice ?? 0), 0));
  const perPlayer    = $derived(podSize > 0 ? draftTotal / podSize : 0);
  const unpricedPicks = $derived(history.filter(e => e.marketPrice == null).length);
  const pricingPending = $derived(history.some(e => e.pricing));
  const historyGroups  = $derived(groupPicks(history));

  // ── Persist history / pod size whenever they change ─────────
  $effect(() => {
    try { localStorage.setItem(HISTORY_KEY, JSON.stringify(history)); } catch { /* storage unavailable */ }
  });
  $effect(() => {
    try { localStorage.setItem(POD_SIZE_KEY, String(podSize)); } catch { /* storage unavailable */ }
  });
  $effect(() => {
    const ids = new Set(packs.map(p => String(p.id)));
    const unchecked = [
      ...[...ids].filter(id => !checked.has(id)),
      // Keep choices for packs not on the page right now (e.g. out of stock).
      ...[..._unchecked].filter(id => !ids.has(id)),
    ];
    const prefs: Prefs = { unchecked, sortKey, sortDir, hideDeselected, recordDraft, draftSaved };
    try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch { /* storage unavailable */ }
  });

  function setPodSize(n: number) {
    if (!Number.isFinite(n)) return;
    podSize = Math.min(MAX_POD_SIZE, Math.max(minPodSize, Math.round(n)));
  }

  function clearHistory() {
    history = [];
    draftSaved = false;
  }

  function formatPrice(p: number | null | undefined): string {
    return p != null ? `$${p.toFixed(2)}` : '—';
  }

  // ── Toggle recording ────────────────────────────────────────
  function setRecordDraft(on: boolean) {
    recordDraft = on;
  }

  // ── Derived ─────────────────────────────────────────────────
  function priceOf(p: Pack): number | null {
    return livePrices[String(p.id)] ?? p.marketPrice ?? null;
  }

  const sortedPacks = $derived.by(() => {
    const copy = hideDeselected ? packs.filter(p => checked.has(String(p.id))) : [...packs];
    const byName = (a: Pack, b: Pack) =>
      a.setName.localeCompare(b.setName) || a.productType.localeCompare(b.productType);
    if (sortKey === 'price') {
      copy.sort((a, b) => {
        const pa = priceOf(a) ?? 0;
        const pb = priceOf(b) ?? 0;
        return sortDir === 'asc' ? pa - pb : pb - pa;
      });
    } else if (sortKey === 'qty') {
      // The number shown on each pack: draft slots still available.
      copy.sort((a, b) => {
        const diff = effectiveQty(a) - effectiveQty(b);
        return (sortDir === 'asc' ? diff : -diff) || byName(a, b);
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

  // ── Stock guard ─────────────────────────────────────────────
  // A draft can only be saved with exactly podSize × 3 picks, so block picking
  // when the selected pool can't fill the remaining slots.
  const remainingPicks  = $derived(Math.max(0, maxPicks - history.length));
  const selectedSlots   = $derived(checkedPacks.reduce((n, p) => n + effectiveQty(p), 0));
  const totalSlots      = $derived(packs.reduce((n, p) => n + effectiveQty(p), 0));
  const shortOfPacks    = $derived(recordDraft && !draftFull && selectedSlots < remainingPicks);

  function packsPerSlot(p: Pack): number {
    if (p.cardsPerPack <= 5) return 3;
    if (p.cardsPerPack <= 8) return 2;
    return 1;
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
      const mp = priceOf(p);
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
    draftSaved = true;
    fetch('/api/drafts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        podSize,
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
        draftSaved = false;
        toast.show('Failed to save draft', 'error');
      }
    }).catch(() => {
      draftSaved = false;
      toast.show('Failed to save draft', 'error');
    });
  }

  // ── Live pricing ────────────────────────────────────────────
  // Fetch the current MTGStocks price for a picked pack so players are
  // charged today's market price, not the last daily refresh.
  // In-flight lookups, so rolling the same pack several times at once
  // only asks MTGStocks once.
  const inflightPrices = new Map<number, Promise<number | null>>();

  function fetchLivePrice(packId: number): Promise<number | null> {
    let p = inflightPrices.get(packId);
    if (!p) {
      p = requestLivePrice(packId).finally(() => inflightPrices.delete(packId));
      inflightPrices.set(packId, p);
    }
    return p;
  }

  async function requestLivePrice(packId: number): Promise<number | null> {
    try {
      const res = await fetch(`/api/collection/${packId}/refresh-price`, { method: 'POST' });
      if (!res.ok) return null;
      const pack = await res.json();
      const price = typeof pack?.marketPrice === 'number' ? pack.marketPrice : null;
      if (price != null) livePrices[String(packId)] = price;
      return price;
    } catch {
      return null;
    }
  }

  async function priceEntry(key: string, packId: number) {
    const live = await fetchLivePrice(packId);
    const idx = history.findIndex(e => e.key === key);
    if (idx === -1) return; // history was cleared meanwhile
    const entry = history[idx];
    const unitPrice = live ?? entry.unitPrice ?? null;
    const packCount = entry.packCount ?? 1;
    history[idx] = {
      ...entry,
      unitPrice,
      marketPrice: unitPrice != null ? unitPrice * packCount : null,
      pricing: false,
    };
    maybeSaveDraft();
  }

  // Save once the draft is full and every pick has its live price.
  function maybeSaveDraft() {
    if (!recordDraft || draftSaved) return;
    if (history.length < maxPicks || history.some(e => e.pricing)) return;
    saveDraft(history);
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

  // Draw one pack from the checked pool, recording it in the draft history
  // when recording is on. Returns null when nothing can be drawn.
  function drawOne(): Pack | null {
    const active = checkedPacks;
    if (active.length === 0) return null;

    const weights = computeWeights(active);
    if (weights.reduce((a, b) => a + b, 0) <= 0) return null;
    const selectedPack = weightedRandom(active, weights);

    if (recordDraft && history.length < maxPicks) {
      const unitPrice = priceOf(selectedPack);
      const packCount = packsPerSlot(selectedPack);
      const key = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
      history = [...history, {
        key,
        packId: selectedPack.id,
        setName: selectedPack.setName,
        productType: selectedPack.productType,
        unitPrice,
        packCount,
        marketPrice: unitPrice != null ? unitPrice * packCount : null,
        pricing: true,
      }];

      const id = String(selectedPack.id);
      if (effectiveQty(selectedPack) === 0) {
        const next = new Set(checked);
        next.delete(id);
        checked = next;
      }

      void priceEntry(key, selectedPack.id);
    }
    return selectedPack;
  }

  function pick() {
    const selectedPack = drawOne();
    if (!selectedPack) return;
    result = { setName: selectedPack.setName, productType: selectedPack.productType };
    scheduleToastDismiss();
  }

  // Roll every remaining slot of the draft in one go.
  function pickAll() {
    if (!recordDraft || shortOfPacks) return;
    const remaining = maxPicks - history.length;
    let drawn = 0;
    for (let i = 0; i < remaining; i++) {
      if (!drawOne()) break;
      drawn++;
    }
    if (drawn === 0) return;
    if (drawn < remaining) {
      toast.show(`Only ${drawn} of ${remaining} packs could be rolled — the pool ran out`, 'error');
    }
    result = { setName: `Rolled ${drawn} pack${drawn === 1 ? '' : 's'}`, productType: '' };
    scheduleToastDismiss();
    // Jump to the history so the total and per-player cost are in view.
    requestAnimationFrame(() => historyEl?.scrollIntoView?.({ behavior: 'smooth', block: 'start' }));
  }
</script>

{#if packs.length === 0}
  <p class="empty">The collection is empty. Ask your admin to add some packs!</p>
{:else}
  <!-- Floating count pill -->
  <div class="count-pill">{checked.size} selected</div>

  <!-- Pod size -->
  <div class="pod">
    <label class="pod__label" for="pod-size">Players</label>
    <input
      id="pod-size"
      class="pod__input"
      type="number"
      inputmode="numeric"
      min={minPodSize}
      max={MAX_POD_SIZE}
      value={podSize}
      disabled={draftSaved}
      onchange={(e) => {
        const el = e.target as HTMLInputElement;
        setPodSize(el.valueAsNumber);
        el.value = String(podSize);
      }}
    />
    <span class="pod__meta">× {PACKS_PER_PLAYER} packs = <strong>{maxPicks}</strong> packs</span>
  </div>

  {#if shortOfPacks}
    <div class="stock-warning" role="alert">
      <strong>Not enough packs.</strong>
      The selected packs fill {selectedSlots} of the {remainingPicks} remaining slots.
      {#if totalSlots >= remainingPicks}
        Select more packs or lower the player count.
      {:else}
        Your whole collection only fills {totalSlots}: lower the player count or add packs.
      {/if}
    </div>
  {/if}

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
      class:sort-btn--active={sortKey === 'qty'}
      onclick={() => onSortClick('qty')}
    >{sortLabel('qty', 'Qty')}</button>
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
            <span class="pack-item__meta">{pack.productType} ({effectiveQty(pack)}{pack.cardsPerPack <= 8 ? ' *' : ''})</span>
          </span>
          <span class="pack-item__price">{formatPrice(priceOf(pack))}</span>
          <span class="pack-item__odds">
            {packOdds != null ? `${packOdds.toFixed(1)}%` : '—'}
          </span>
        </label>
      </li>
    {/each}
  </ul>

  <!-- Non-standard pack size footnote -->
  {#if checkedPacks.some(p => p.cardsPerPack <= 8)}
    <ul class="pack-footnote">
      {#each checkedPacks.filter(p => p.cardsPerPack <= 8) as p}
        {@const slots = packsPerSlot(p)}
        {@const unit = priceOf(p)}
        <li>* {p.setName} {p.productType}: {p.cardsPerPack} cards/pack, {slots} packs per slot{unit != null ? ` (${slots} × ${formatPrice(unit)} = ${formatPrice(unit * slots)} per slot)` : ''}</li>
      {/each}
    </ul>
  {/if}

  <!-- Pick row -->
  <div class="pick-row">
    <button
      class="btn btn--primary btn--large"
      disabled={noneChecked || draftFull || shortOfPacks}
      onclick={pick}
    >🎲 Pick a Pack</button>
    <button
      class="btn btn--secondary btn--large"
      disabled={noneChecked || draftFull || !recordDraft || shortOfPacks}
      title={recordDraft ? `Roll the remaining ${maxPicks - history.length} packs` : 'Turn on "Record draft" to roll all'}
      onclick={pickAll}
    >🎲 Roll all ({maxPicks - history.length})</button>
    <label class="history-toggle">
      <input type="checkbox" checked={recordDraft} disabled={draftFull} onchange={(e) => setRecordDraft((e.target as HTMLInputElement).checked)} />
      Record draft
    </label>
    {#if draftFull}
      <span class="draft-full">Draft complete ({history.length}/{maxPicks})</span>
    {:else if recordDraft && history.length > 0}
      <span class="draft-full">{history.length}/{maxPicks}</span>
    {/if}
  </div>

  <!-- Result toast -->
  <!-- Always-present live region so screen readers announce each pick. -->
  <div role="status">
    {#if result}
      <button type="button" class="toast" title="Dismiss" onclick={dismissResult}>
        <span class="toast__text">{result.setName}{result.productType ? ` · ${result.productType}` : ''}</span>
      </button>
    {/if}
  </div>

  <!-- History -->
  {#if recordDraft && history.length > 0}
    <div class="history" bind:this={historyEl}>
      <div class="history__header">
        <span class="history__title">Draft history ({history.length}/{maxPicks})</span>
        <button class="history__clear" onclick={clearHistory}>Clear</button>
      </div>
      <div class="cost">
        <div class="cost__row">
          <span>Pack total{draftFull ? '' : ' so far'}</span>
          <span class="cost__value">{formatPrice(draftTotal)}</span>
        </div>
        <div class="cost__row cost__row--main">
          <span>Per player ({podSize})</span>
          <span class="cost__value">{formatPrice(perPlayer)}</span>
        </div>
        {#if pricingPending}
          <div class="cost__note">Fetching current prices…</div>
        {:else if unpricedPicks > 0}
          <div class="cost__note">{unpricedPicks} pick{unpricedPicks === 1 ? '' : 's'} without a price — not included</div>
        {/if}
      </div>
      <ol class="history__list">
        {#each historyGroups as g (g.key)}
          <li class="history__item">
            <span class="history__num">{g.packs}×</span>
            <span class="history__pack-info">
              <span class="history__pack-name">{g.setName}</span>
              <span class="history__pack-set">{g.productType}{g.packs !== g.slots ? ` · ${g.slots} slot${g.slots === 1 ? '' : 's'}` : ''}</span>
            </span>
            <span class="history__pack-price" class:history__pack-price--pending={g.pricing}>
              {#if g.qty > 1 && g.each != null}
                <span class="history__pack-mult">{g.qty}× {formatPrice(g.each)} =</span>
              {/if}
              {formatPrice(g.total)}
            </span>
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

  /* ── Pod size ──────────────────────────────────────────────── */
  .pod {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-bottom: 0.9rem;
  }
  .pod__label { font-size: 0.88rem; font-weight: 500; }
  .pod__input {
    width: 4.5rem;
    padding: 0.35rem 0.5rem;
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.95rem;
    font-variant-numeric: tabular-nums;
  }
  .pod__input:focus { outline: none; border-color: var(--color-accent); }
  .pod__input:disabled { opacity: 0.6; }
  .pod__meta { font-size: 0.82rem; color: var(--color-text-muted); }
  .pod__meta strong { color: var(--color-text); }

  /* ── Cost summary ──────────────────────────────────────────── */
  .cost {
    padding: 0.6rem 0.85rem;
    border-bottom: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.88rem;
  }
  .cost__row {
    display: flex;
    justify-content: space-between;
    color: var(--color-text-muted);
  }
  .cost__row--main { color: var(--color-text); font-weight: 600; font-size: 1rem; }
  .cost__row--main .cost__value { color: var(--color-accent); }
  .cost__value { font-variant-numeric: tabular-nums; }
  .cost__note { font-size: 0.75rem; color: var(--color-text-muted); opacity: 0.85; }

  /* ── Stock warning ─────────────────────────────────────────── */
  .stock-warning {
    margin-bottom: 0.9rem;
    padding: 0.6rem 0.8rem;
    border: 1px solid var(--color-danger);
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--color-danger) 12%, transparent);
    font-size: 0.85rem;
    line-height: 1.4;
  }
  .stock-warning strong { color: var(--color-danger); }

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

  /* Phones: a solid full-width bar so the buttons never sit on top of the list. */
  @media (max-width: 639px) {
    .pick-row {
      left: 0;
      right: 0;
      bottom: 0;
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 0.5rem 0.6rem;
      padding: 0.65rem 1rem calc(0.65rem + env(safe-area-inset-bottom));
      background: var(--color-bg);
      border-top: 1px solid var(--color-border);
      box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.3);
    }
    .pick-row .btn--large {
      padding: 0.75rem 0.5rem;
      font-size: 0.98rem;
      white-space: nowrap;
    }
    .pick-row .draft-full { justify-self: end; }
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
    font: inherit;
    text-align: left;
    width: auto;
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
    /* Land below the result toast and the floating count pill when scrolled to. */
    scroll-margin-top: 8rem;
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

  .history__pack-price--pending { opacity: 0.5; }
  .history__pack-mult { font-size: 0.7rem; opacity: 0.8; margin-right: 0.15rem; }

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

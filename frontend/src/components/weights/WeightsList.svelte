<script lang="ts">
  import SetCard from './SetCard.svelte';

  let { packs, settings }: {
    packs: any[];
    settings: {
      priceFloor: number;
      priceCap: number;
      quantityCap: number;
      packWeights: Record<string, number>;
    };
  } = $props();

  // ── State ──────────────────────────────────────────────────
  let priceFloor  = $state(settings.priceFloor  > 0 ? settings.priceFloor  : 0);
  let priceCap    = $state(settings.priceCap    > 0 ? settings.priceCap    : 0);
  let quantityCap = $state(settings.quantityCap > 0 ? settings.quantityCap : 0);

  // Initialize ALL packs so we always send the full set on save (no dynamic key additions needed)
  const initMultipliers = (): Record<string, number> => {
    const m: Record<string, number> = {};
    for (const p of packs) {
      m[String(p.id)] = (settings.packWeights ?? {})[String(p.id)] ?? 0;
    }
    return m;
  };
  let multipliers = $state<Record<string, number>>(initMultipliers());
  let sortKey     = $state<'name' | 'price' | 'weight'>('name');
  let sortDir     = $state<'asc' | 'desc'>('asc');
  let saving      = $state(false);

  // ── Helpers ────────────────────────────────────────────────
  function toNum(v: string)    { const n = parseFloat(v); return isNaN(n) || n <= 0 ? 0 : n; }
  function toInt(v: string)    { const n = parseInt(v);   return isNaN(n) || n <= 0 ? 0 : n; }
  function updateMult(id: string, value: number) { multipliers[id] = value; }

  // ── Odds computation ───────────────────────────────────────
  function computeOdds(
    allPacks: any[],
    floor: number,
    cap: number,
    qtyCap: number,
    mults: Record<string, number>,
  ): Record<string, number> {
    const capPrice = (p: number) => {
      if (floor > 0 && p < floor) p = floor;
      if (cap   > 0 && p > cap)   p = cap;
      return p;
    };
    const capQty = (q: number) => qtyCap > 0 && q > qtyCap ? qtyCap : q;

    const rawPrices = allPacks.map(p => {
      const price = p.marketPrice;
      return price && price > 0 ? capPrice(price) : null;
    });
    const validPrices = rawPrices.filter((p): p is number => p !== null);
    const avgPrice = validPrices.length
      ? validPrices.reduce((a, b) => a + b, 0) / validPrices.length
      : 1;

    const pw = allPacks.map((_, i) => 1 / (rawPrices[i] ?? avgPrice));
    const sw = allPacks.map(p => { const q = capQty(p.quantity); return q > 0 ? 1 / q : 0; });

    const priceTotal    = pw.reduce((a, b) => a + b, 0);
    const scarcityTotal = sw.reduce((a, b) => a + b, 0);

    const weights = allPacks.map((p, i) => {
      if (sw[i] === 0) return 0;
      const base = Math.min(
        priceTotal    > 0 ? pw[i] / priceTotal    : 0,
        scarcityTotal > 0 ? sw[i] / scarcityTotal : 0,
      );
      const mult = mults[String(p.id)] ?? 0;
      return base * Math.max(0, 1 + mult);
    });

    const total = weights.reduce((a, b) => a + b, 0);
    const result: Record<string, number> = {};
    allPacks.forEach((p, i) => {
      result[String(p.id)] = total > 0 ? (weights[i] / total) * 100 : 0;
    });
    return result;
  }

  // ── Sorted sets computation ────────────────────────────────
  function computeSortedSets(
    allPacks: any[],
    currentOdds: Record<string, number>,
    key: typeof sortKey,
    dir: typeof sortDir,
  ): [string, any[]][] {
    const setMap = new Map<string, any[]>();
    for (const pack of allPacks) {
      if (!setMap.has(pack.setName)) setMap.set(pack.setName, []);
      setMap.get(pack.setName)!.push(pack);
    }

    const packPrice = (p: any) => (p.marketPrice ?? 0) as number;
    const packOdds  = (p: any) => currentOdds[String(p.id)] ?? 0;
    const groupMax  = (packList: any[], fn: (p: any) => number) =>
      Math.max(0, ...packList.map(fn));

    const sets = Array.from(setMap.entries());
    sets.sort(([nameA, packsA], [nameB, packsB]) => {
      if (key === 'name') {
        return dir === 'asc' ? nameA.localeCompare(nameB) : nameB.localeCompare(nameA);
      }
      const fn   = key === 'price' ? packPrice : packOdds;
      const diff = groupMax(packsA, fn) - groupMax(packsB, fn);
      return dir === 'asc' ? diff : -diff;
    });

    if (key !== 'name') {
      const fn = key === 'price' ? packPrice : packOdds;
      return sets.map(([name, packList]) => [
        name,
        [...packList].sort((a, b) => {
          const diff = fn(a) - fn(b);
          return dir === 'asc' ? diff : -diff;
        }),
      ]);
    }
    return sets;
  }

  // ── Derived ────────────────────────────────────────────────
  const odds       = $derived(computeOdds(packs, priceFloor, priceCap, quantityCap, multipliers));
  const sortedSets = $derived(computeSortedSets(packs, odds, sortKey, sortDir));

  // ── Sort ───────────────────────────────────────────────────
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

  // ── Save ───────────────────────────────────────────────────
  async function save() {
    saving = true;
    const res = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ priceFloor, priceCap, quantityCap, packWeights: { ...multipliers } }),
    });
    saving = false;
    if (res.ok) {
      (window as any).showToast?.('Settings saved', 'success');
    } else {
      (window as any).showToast?.(`Failed to save: ${res.status} ${res.statusText}`, 'error');
    }
  }
</script>

<!-- ── Global caps ──────────────────────────────────────── -->
<div class="caps">
  <div class="caps__field">
    <label class="caps__label" for="price-floor">Price floor</label>
    <div class="caps__input-wrap">
      <span class="caps__prefix">$</span>
      <input
        id="price-floor"
        class="caps__input"
        type="number"
        min="0"
        step="1"
        placeholder="None"
        value={priceFloor > 0 ? priceFloor : ''}
        oninput={(e) => { priceFloor = toNum((e.target as HTMLInputElement).value); }}
      />
    </div>
    <span class="caps__hint">Cheap packs treated as this price</span>
  </div>
  <div class="caps__field">
    <label class="caps__label" for="price-cap">Price cap</label>
    <div class="caps__input-wrap">
      <span class="caps__prefix">$</span>
      <input
        id="price-cap"
        class="caps__input"
        type="number"
        min="0"
        step="1"
        placeholder="None"
        value={priceCap > 0 ? priceCap : ''}
        oninput={(e) => { priceCap = toNum((e.target as HTMLInputElement).value); }}
      />
    </div>
    <span class="caps__hint">Expensive packs treated as this price</span>
  </div>
  <div class="caps__field">
    <label class="caps__label" for="qty-cap">Pack qty cap</label>
    <input
      id="qty-cap"
      class="caps__input"
      type="number"
      min="0"
      step="1"
      placeholder="None"
      value={quantityCap > 0 ? quantityCap : ''}
      oninput={(e) => { quantityCap = toInt((e.target as HTMLInputElement).value); }}
    />
    <span class="caps__hint">High-qty packs treated as this qty</span>
  </div>
  <button class="btn-save" disabled={saving} onclick={save}>Save</button>
</div>

<!-- ── Sort bar ────────────────────────────────────────── -->
<div class="pack-sort">
  <span class="pack-sort__label">Sort:</span>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'name'}
    onclick={() => onSortClick('name')}
  >{sortLabel('name', 'Set')}</button>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'price'}
    onclick={() => onSortClick('price')}
  >{sortLabel('price', 'Price')}</button>
  <button
    class="sort-btn"
    class:sort-btn--active={sortKey === 'weight'}
    onclick={() => onSortClick('weight')}
  >{sortLabel('weight', 'Weight')}</button>
</div>

<!-- ── Column headers ─────────────────────────────────── -->
<div class="col-headers">
  <span>Qty</span>
  <span>Price</span>
  <span>Odds</span>
  <span>Weight</span>
</div>

<!-- ── Pack list ───────────────────────────────────────── -->
<div class="pack-list">
  {#each sortedSets as [setName, setPacks] (setName)}
    <SetCard
      {setName}
      packs={setPacks}
      packOdds={odds}
      {multipliers}
      onMultChange={updateMult}
    />
  {/each}
</div>

<style>
  /* ── Caps ──────────────────────────────────────────────── */
  .caps {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.75rem 1.25rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.9rem 1.1rem;
    margin-bottom: 1.25rem;
  }

  .caps__field {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .caps__label {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .caps__input-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }

  .caps__prefix {
    position: absolute;
    left: 0.55rem;
    color: var(--color-text-muted);
    font-size: 0.88rem;
    pointer-events: none;
  }

  .caps__input {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.88rem;
    padding: 0.35rem 0.55rem;
    width: 7rem;
  }
  .caps__input:focus { outline: none; border-color: var(--color-accent); }
  .caps__input-wrap .caps__input { padding-left: 1.3rem; }

  .caps__hint {
    font-size: 0.72rem;
    color: var(--color-text-muted);
    opacity: 0.7;
  }

  .btn-save {
    background: var(--color-accent);
    color: #fff;
    border: none;
    border-radius: var(--radius);
    padding: 0.4rem 1rem;
    font-size: 0.88rem;
    font-weight: 500;
    cursor: pointer;
    align-self: flex-end;
  }
  .btn-save:hover { opacity: 0.85; }
  .btn-save:disabled { opacity: 0.5; cursor: default; }

  /* ── Sort bar ──────────────────────────────────────────── */
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

  /* ── Column headers ────────────────────────────────────── */
  .col-headers {
    display: flex;
    justify-content: space-between;
    padding: 0.25rem 0.75rem 0.3rem;
    font-size: 0.68rem;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.7;
  }

  /* ── Pack list ─────────────────────────────────────────── */
  .pack-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
</style>

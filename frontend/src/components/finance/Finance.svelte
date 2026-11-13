<script lang="ts">
  import { untrack } from 'svelte';
  import { toast } from '../../lib/toast.svelte';
  import { send } from '../../lib/send';
  import PurchaseForm from './PurchaseForm.svelte';
  import SaleForm from './SaleForm.svelte';
  import PurchaseEdit from './PurchaseEdit.svelte';
  import SaleEdit from './SaleEdit.svelte';
  import DraftEdit from './DraftEdit.svelte';
  import { packCost, discount, verdict, formatDiscount, formatReturn, byNewest, type Purchase } from '../../lib/purchases';
  import { portfolio, activity, type Disposal } from '../../lib/portfolio';

  interface Pack {
    id: number;
    setName: string;
    productType: string;
    quantity: number;
    marketPrice: number | null;
  }

  let { purchases: initialPurchases, disposals: initialDisposals, packs: initialPacks, canEdit = false }: {
    purchases: Purchase[];
    disposals: Disposal[];
    packs: Pack[];
    canEdit?: boolean;
  } = $props();

  let purchases = $state<Purchase[]>(untrack(() => initialPurchases.map(p => ({ ...p }))));
  let disposals = $state<Disposal[]>(untrack(() => initialDisposals.map(d => ({ ...d }))));
  let packs     = $state<Pack[]>(untrack(() => initialPacks.map(p => ({ ...p }))));

  const pf      = $derived(portfolio(purchases, disposals, packs));
  const rows    = $derived(activity(purchases, disposals));
  const loggedIds = $derived(new Set(purchases.map(p => p.packId)));
  const packOptions = $derived(
    [...packs].sort((a, b) => a.setName.localeCompare(b.setName) || a.productType.localeCompare(b.productType)),
  );

  let openForm = $state<'buy' | 'sale' | null>(null);
  let editing  = $state<string | null>(null); // activity row key

  // ── Keep stock in step with sales (for the sale form's limits) ──
  function moveStock(packId: number, delta: number) {
    const p = packs.find(x => x.id === packId);
    if (p) p.quantity = Math.max(0, p.quantity + delta);
  }

  function onPurchase(p: Purchase, addedToStock: boolean) {
    purchases = [p, ...purchases].sort(byNewest);
    if (addedToStock) moveStock(p.packId, p.quantity);
  }
  function onPurchaseSaved(p: Purchase) {
    purchases = purchases.map(x => (x.id === p.id ? p : x)).sort(byNewest);
    editing = null;
  }
  function onSale(d: Disposal) {
    disposals = [d, ...disposals];
    moveStock(d.packId, -d.quantity);
  }
  function onSaleSaved(d: Disposal) {
    const before = disposals.find(x => x.id === d.id);
    if (before) moveStock(d.packId, before.quantity - d.quantity);
    disposals = disposals.map(x => (x.id === d.id ? d : x));
    editing = null;
  }
  function onDraftSaved(records: Disposal[]) {
    const byId = new Map(records.map(r => [r.id, r]));
    disposals = disposals.map(x => byId.get(x.id) ?? x);
    editing = null;
  }

  function deletePurchase(p: Purchase) {
    toast.confirm(`Delete the ${p.purchasedOn} purchase of ${p.setName} ${p.productType}? Stock is not changed.`, async () => {
      if (await send(`/api/purchases/${p.id}`, 'DELETE', null, 'Failed to delete')) {
        purchases = purchases.filter(x => x.id !== p.id);
      }
    });
  }
  function undoSale(d: Disposal) {
    toast.confirm(`Undo the ${d.disposedOn} sale of ${d.quantity} ${d.setName} ${d.productType}? The packs go back into stock.`, async () => {
      if (await send(`/api/sales/${d.id}`, 'DELETE', null, 'Failed to undo sale')) {
        disposals = disposals.filter(x => x.id !== d.id);
        moveStock(d.packId, d.quantity);
      }
    });
  }

  // ── Formatting ─────────────────────────────────────────────
  const money  = (n: number | null | undefined) => (n != null ? `$${n.toFixed(2)}` : '—');
  const signed = (n: number) => `${n >= 0 ? '+' : '−'}$${Math.abs(n).toFixed(2)}`;
  const tone   = (n: number | null | undefined) => (n == null ? 'none' : n > 0.005 ? 'good' : n < -0.005 ? 'bad' : 'fair');
  const packsLabel = (n: number) => `${n} pack${n === 1 ? '' : 's'}`;
  const sum = (list: Disposal[], f: (d: Disposal) => number) => list.reduce((s, d) => s + f(d), 0);

  function gainOf(list: Disposal[]): number | null {
    let g: number | null = null;
    for (const d of list) {
      const r = pf.results.get(d.id)?.gain;
      if (r != null) g = (g ?? 0) + r;
    }
    return g;
  }
  const unknownOf = (list: Disposal[]) => list.reduce((n, d) => n + (pf.results.get(d.id)?.unknownPacks ?? 0), 0);
</script>

<div class="finance">
  <!-- ── Overview ─────────────────────────────────────────── -->
  {#if pf.boughtPacks > 0}
    <p class="invested">
      Invested <strong>{money(pf.invested)}</strong> in {packsLabel(pf.boughtPacks)} logged{#if pf.purchaseFees > 0}&nbsp;(incl. {money(pf.purchaseFees)} tax & shipping){/if}
    </p>
  {/if}
  <div class="summary">
    <div class="card">
      <span class="card__label">Holdings</span>
      <span class="card__value">{money(pf.value)}</span>
      <span class="card__sub">
        {packsLabel(pf.stockPacks)} in stock at today's market{#if pf.uncostedPacks > 0}&nbsp;· {pf.uncostedPacks} with no logged cost{/if}
      </span>
    </div>
    <div class="card">
      <span class="card__label">Unrealized</span>
      <span class="card__value tone--{tone(pf.unrealized)}">
        {signed(pf.unrealized)}<span class="card__pct">{formatReturn(pf.unrealizedPct)}</span>
      </span>
      <span class="card__sub">packs in stock with a logged cost of {money(pf.cost)}</span>
    </div>
    <div class="card">
      <span class="card__label">Realized</span>
      <span class="card__value tone--{tone(pf.realized)}">{signed(pf.realized)}</span>
      <span class="card__sub">
        recorded sales & drafts · {money(pf.proceeds)} collected{#if pf.unknownPacks > 0}&nbsp;· {packsLabel(pf.unknownPacks)} with no logged cost{/if}
      </span>
    </div>
    <div class="card">
      <span class="card__label">Opened & kept</span>
      <span class="card__value">{money(pf.openedCost)}</span>
      <span class="card__sub">cost of {packsLabel(pf.openedPacks)} logged but no longer in stock · not a loss</span>
    </div>
  </div>

  <!-- ── Holdings ─────────────────────────────────────────── -->
  {#if pf.positions.length > 0}
    <section class="section">
      <h2 class="section__title">Holdings</h2>
      <div class="table-wrap">
        <table class="holdings">
          <thead>
            <tr>
              <th>Pack</th>
              <th class="num">In stock</th>
              <th class="num">Avg cost</th>
              <th class="num">Market</th>
              <th class="num">Value</th>
              <th class="num">Unrealized</th>
              <th class="num">Realized</th>
            </tr>
          </thead>
          <tbody>
            {#each pf.positions as h (h.packId)}
              <tr class:closed={h.stock === 0}>
                <td class="holdings__name">
                  {h.setName} <span class="muted">{h.productType}</span>
                  {#if h.bought > 0 || h.uncosted > 0}
                    <span class="holdings__detail">
                      {[
                        h.bought > 0 ? `${h.bought} bought` : '',
                        h.disposed > 0 ? `${h.disposed} sold/drafted` : '',
                        h.opened > 0 ? `${h.opened} opened` : '',
                        h.uncosted > 0 && h.bought > 0 ? `${h.uncosted} with no logged cost` : '',
                      ].filter(Boolean).join(' · ')}
                    </span>
                  {/if}
                </td>
                <td class="num">{h.stock}</td>
                <td class="num">{h.costed > 0 ? money(h.avgCost) : '—'}</td>
                <td class="num">{money(h.currentPrice)}</td>
                <td class="num">{money(h.value)}</td>
                <td class="num tone--{tone(h.unrealized)}">
                  {h.unrealized != null ? signed(h.unrealized) : '—'}
                  {#if h.unrealizedPct != null}<span class="muted">{formatReturn(h.unrealizedPct)}</span>{/if}
                </td>
                <td class="num tone--{tone(h.realized)}">{h.disposed > 0 ? signed(h.realized) : '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <p class="note">
        In stock is what Manage Collection shows. Avg cost and unrealized cover packs with a logged purchase,
        without tax & shipping (market prices don't include them). Logged packs that are no longer in stock and
        weren't recorded as sold or drafted count as opened.
      </p>
    </section>
  {/if}

  <!-- ── Activity ─────────────────────────────────────────── -->
  <section class="section">
    <div class="section__header">
      <h2 class="section__title">Activity</h2>
      {#if canEdit}
        <div class="section__actions">
          <button class="btn-link" class:btn-link--on={openForm === 'buy'}
            onclick={() => (openForm = openForm === 'buy' ? null : 'buy')}>+ Log a purchase</button>
          <button class="btn-link" class:btn-link--on={openForm === 'sale'}
            onclick={() => (openForm = openForm === 'sale' ? null : 'sale')}>+ Record a sale</button>
        </div>
      {/if}
    </div>

    {#if canEdit && openForm === 'buy'}
      <PurchaseForm packs={packOptions} {loggedIds} oncreated={onPurchase} />
    {:else if canEdit && openForm === 'sale'}
      <SaleForm packs={packOptions} oncreated={onSale} />
    {/if}

    {#if rows.length === 0}
      <p class="empty">
        Nothing yet. Enter what you paid when adding packs on Manage Collection, or log past purchases here.
        Approved drafts show up here automatically.
      </p>
    {:else}
      <ul class="log">
        {#each rows as row (row.key)}
          <li class="log__item">
            {#if row.type === 'buy'}
              {@const p = row.purchase}
              {@const each = packCost(p) / p.quantity}
              {@const d = discount(each, p.currentPrice)}
              <div class="log__main">
                <span class="tag tag--buy">Buy</span>
                <span class="log__date">{p.purchasedOn}</span>
                <span class="log__name">{p.setName} <span class="muted">{p.productType}</span></span>
                {#if canEdit && editing !== row.key}
                  <button class="btn-row" onclick={() => (editing = row.key)}>Edit</button>
                  <button class="btn-row btn-row--danger" title="Delete purchase" onclick={() => deletePurchase(p)}>✕</button>
                {/if}
              </div>
              {#if editing === row.key}
                <PurchaseEdit purchase={p} onsaved={onPurchaseSaved} oncancel={() => (editing = null)} />
              {/if}
              <div class="log__nums">
                <span>
                  <strong>{money(p.totalPrice)}</strong> for {packsLabel(p.quantity)}{#if p.quantity > 1}&nbsp;({money(each)} each){/if}{#if p.fees > 0}&nbsp;· incl. {money(p.fees)} tax & shipping{/if}
                </span>
                <span title="Market price per pack today">
                  market {money(p.currentPrice)}
                  {#if d != null}<span class="tone--{verdict(d) ?? 'none'} strong">{formatDiscount(d)}</span>{/if}
                </span>
              </div>
            {:else if row.type === 'sale'}
              {@const s = row.disposal}
              {@const r = pf.results.get(s.id)}
              <div class="log__main">
                <span class="tag tag--sale">Sale</span>
                <span class="log__date">{s.disposedOn}</span>
                <span class="log__name">{s.setName} <span class="muted">{s.productType}</span></span>
                {#if canEdit && editing !== row.key}
                  <button class="btn-row" onclick={() => (editing = row.key)}>Edit</button>
                  <button class="btn-row btn-row--danger" title="Undo sale" onclick={() => undoSale(s)}>✕</button>
                {/if}
              </div>
              {#if editing === row.key}
                <SaleEdit sale={s} onsaved={onSaleSaved} oncancel={() => (editing = null)} />
              {/if}
              <div class="log__nums">
                <span>
                  <strong>{money(s.proceeds)}</strong> for {packsLabel(s.quantity)}{#if s.quantity > 1}&nbsp;({money(s.proceeds / s.quantity)} each){/if}{#if s.fees > 0}&nbsp;· {money(s.fees)} fees{/if}
                </span>
                <span class="tone--{tone(r?.gain)} strong">
                  {r?.gain != null ? `${signed(r.gain)} gain` : 'no logged cost'}
                </span>
              </div>
            {:else}
              {@const list = row.disposals}
              {@const collected = sum(list, d => d.proceeds)}
              {@const market = sum(list, d => d.marketValue ?? 0)}
              {@const gain = gainOf(list)}
              {@const unknown = unknownOf(list)}
              <div class="log__main">
                <span class="tag tag--draft">Draft</span>
                <span class="log__date">{row.date}</span>
                <span class="log__name">
                  {packsLabel(sum(list, d => d.quantity))} opened
                  <span class="muted">{list.map(d => d.setName).join(', ')}</span>
                </span>
                {#if canEdit && row.draftId != null && editing !== row.key}
                  <button class="btn-row" onclick={() => (editing = row.key)}>Edit</button>
                {/if}
              </div>
              {#if editing === row.key && row.draftId != null}
                <DraftEdit draftId={row.draftId} {collected} onsaved={onDraftSaved} oncancel={() => (editing = null)} />
              {/if}
              <div class="log__nums">
                <span>
                  <strong>{money(collected)}</strong> collected{#if market > 0}&nbsp;· market {money(market)}{/if}
                </span>
                <span class="tone--{tone(gain)} strong">
                  {gain != null ? `${signed(gain)} gain` : 'no logged cost'}
                </span>
                {#if gain != null && unknown > 0}
                  <span>{packsLabel(unknown)} with no logged cost</span>
                {/if}
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .muted { color: var(--color-text-muted); font-size: 0.85em; }
  .strong { font-weight: 600; }
  .tone--good { color: var(--color-success); }
  .tone--bad  { color: var(--color-danger); }
  .tone--fair, .tone--none { color: inherit; }

  /* ── Summary ──────────────────────────────────────────────── */
  .summary {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.6rem;
    margin-bottom: 1.75rem;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.8rem 0.9rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: var(--color-surface);
    min-width: 0;
  }
  .card__label { font-size: 0.75rem; color: var(--color-text-muted); }
  .card__value { font-size: 1.3rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .card__pct { font-size: 0.82rem; font-weight: 500; margin-left: 0.35rem; }
  .card__sub { font-size: 0.72rem; color: var(--color-text-muted); }

  @media (max-width: 799px) { .summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
  @media (max-width: 479px) { .summary { grid-template-columns: 1fr; } }

  /* ── Sections ─────────────────────────────────────────────── */
  .section { margin-bottom: 2rem; }
  .section__header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    margin-bottom: 0.75rem;
  }
  .section__title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-text-muted);
    margin: 0 0 0.75rem;
  }
  .section__header .section__title { margin: 0; }
  .section__actions { display: flex; gap: 1rem; }
  .note, .empty {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    opacity: 0.85;
    margin: 0.5rem 0 0;
  }
  .empty { font-size: 0.85rem; padding: 1rem 0; }

  /* ── Holdings ─────────────────────────────────────────────── */
  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
  }
  .holdings {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }
  .holdings th {
    text-align: left;
    font-size: 0.72rem;
    font-weight: 500;
    color: var(--color-text-muted);
    background: var(--color-surface);
    padding: 0.45rem 0.7rem;
    border-bottom: 1px solid var(--color-border);
    white-space: nowrap;
  }
  .holdings td {
    padding: 0.45rem 0.7rem;
    border-bottom: 1px solid var(--color-border);
    white-space: nowrap;
  }
  .holdings tr:last-child td { border-bottom: none; }
  .holdings .num { text-align: right; }
  .holdings .num .muted { margin-left: 0.25rem; }
  .holdings__name { white-space: normal !important; min-width: 10rem; }
  .holdings__detail { display: block; font-size: 0.7rem; color: var(--color-text-muted); }
  .invested { font-size: 0.85rem; color: var(--color-text-muted); margin: 0 0 0.6rem; }
  .invested strong { color: var(--color-text); }
  .holdings tr.closed td { opacity: 0.55; }

  /* ── Activity ─────────────────────────────────────────────── */
  .btn-link {
    background: none;
    border: none;
    color: var(--color-accent);
    font-size: 0.85rem;
    cursor: pointer;
    padding: 0;
  }
  .btn-link--on { text-decoration: underline; }
  .log {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .log__item {
    padding: 0.55rem 0.75rem;
    border-bottom: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .log__item:last-child { border-bottom: none; }
  .log__main { display: flex; align-items: baseline; gap: 0.6rem; min-width: 0; }
  .log__date {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }
  .log__name {
    flex: 1;
    min-width: 0;
    font-size: 0.88rem;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .log__nums {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 1rem;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
  }
  .log__nums strong { color: var(--color-text); font-weight: 600; }
  .tag {
    font-size: 0.65rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.05rem 0.4rem;
    border-radius: var(--radius);
    border: 1px solid currentColor;
    flex-shrink: 0;
    align-self: center;
  }
  .tag--buy   { color: var(--color-accent); }
  .tag--sale  { color: var(--color-success); }
  .tag--draft { color: var(--color-text-muted); }

  /* ── Forms (shared by the form components) ────────────────── */
  .finance :global(.form) {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 0.6rem 0.75rem;
    padding: 0.75rem;
    margin-bottom: 1rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: var(--color-surface);
  }
  .finance :global(.form--inline) { padding: 0.5rem 0 0.25rem; margin: 0; border: none; background: none; }
  .finance :global(.form__actions) { display: flex; align-items: center; gap: 0.5rem; align-self: center; }
  .finance :global(.form__hint) {
    flex-basis: 100%;
    margin: 0;
    font-size: 0.72rem;
    color: var(--color-text-muted);
    opacity: 0.85;
  }
  .finance :global(.field) {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }
  .finance :global(.field--wide) { flex: 1 1 100%; }
  .finance :global(.field input), .finance :global(.field select) {
    padding: 0.35rem 0.5rem;
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }
  .finance :global(.field input[type='number']) { width: 6.5rem; }
  .finance :global(.field input:focus), .finance :global(.field select:focus) { outline: none; border-color: var(--color-accent); }
  .finance :global(.check) {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.82rem;
    color: var(--color-text-muted);
    align-self: center;
  }
  .finance :global(.check input) { accent-color: var(--color-accent); }
  .finance :global(.btn-save) {
    background: var(--color-accent);
    color: white;
    border: none;
    border-radius: var(--radius);
    padding: 0.45rem 1.1rem;
    font-size: 0.9rem;
    cursor: pointer;
    align-self: center;
  }
  .finance :global(.btn-save:disabled) { opacity: 0.5; cursor: not-allowed; }
  .finance :global(.btn-row) {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: var(--radius);
    padding: 0.1rem 0.45rem;
    font-size: 0.72rem;
    cursor: pointer;
    flex-shrink: 0;
  }
  .finance :global(.btn-row:hover) { border-color: var(--color-accent); color: var(--color-accent); }
  .finance :global(.btn-row--danger:hover) { border-color: var(--color-danger); color: var(--color-danger); }
</style>

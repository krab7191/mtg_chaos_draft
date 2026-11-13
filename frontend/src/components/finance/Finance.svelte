<script lang="ts">
  import { untrack } from 'svelte';
  import { toast } from '../../lib/toast.svelte';
  import PriceInput from './PriceInput.svelte';
  import {
    packCost, discount, verdict, formatDiscount, returnPct, formatReturn,
    totals, holdings, priceFields, byNewest, localToday,
    type Purchase, type PriceMode,
  } from '../../lib/purchases';

  interface PackOption {
    id: number;
    setName: string;
    productType: string;
  }

  let { purchases: initial, packs, canEdit = false }: {
    purchases: Purchase[];
    packs: PackOption[];
    canEdit?: boolean;
  } = $props();

  let purchases = $state<Purchase[]>(untrack(() => initial.map(p => ({ ...p }))));

  // ── Overview ───────────────────────────────────────────────
  const sum      = $derived(totals(purchases));
  const gain     = $derived(sum.valueNow != null ? sum.valueNow - sum.costWithValueNow : null);
  const gainPct  = $derived(sum.valueNow != null ? returnPct(sum.valueNow, sum.costWithValueNow) : null);
  const netGain  = $derived(sum.valueNow != null ? sum.valueNow - sum.paidWithValueNow : null);
  const positions = $derived(holdings(purchases));

  // Packs with nothing logged yet: what's left to back-fill from receipts.
  const loggedIds     = $derived(new Set(purchases.map(p => p.packId)));
  const unloggedCount = $derived(packs.filter(p => !loggedIds.has(p.id)).length);
  const packOptions   = $derived(
    [...packs].sort((a, b) => a.setName.localeCompare(b.setName) || a.productType.localeCompare(b.productType)),
  );

  // ── Log a purchase ─────────────────────────────────────────
  let formOpen   = $state(false);
  let packId     = $state<number | ''>('');
  let quantity   = $state(1);
  let paid       = $state<number | null>(null);
  let priceMode  = $state<PriceMode>('total');
  let fees       = $state<number | null>(null);
  let date       = $state(localToday());
  let addToStock = $state(false);
  let saving     = $state(false);

  const canSubmit = $derived(
    packId !== '' && quantity >= 1 && paid != null && paid >= 0 && (fees == null || fees >= 0) && !!date && !saving,
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    saving = true;
    try {
      const res = await fetch('/api/purchases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          packId, quantity, ...priceFields(priceMode, paid!), fees: fees ?? 0, purchasedOn: date, addToStock,
        }),
      });
      if (!res.ok) {
        toast.show(`Failed to log purchase: ${(await res.text()).trim() || res.status}`, 'error');
        return;
      }
      const created: Purchase = await res.json();
      purchases = [created, ...purchases].sort(byNewest);
      toast.show('Purchase logged', 'success');
      // Keep the date for the next line of the same receipt.
      packId = '';
      paid = null;
      fees = null;
      quantity = 1;
    } catch {
      toast.show('Failed to log purchase', 'error');
    } finally {
      saving = false;
    }
  }

  // ── Edit in place ──────────────────────────────────────────
  let editingId = $state<number | null>(null);
  let eQty      = $state(1);
  let ePaid     = $state<number | null>(null);
  let eMode     = $state<PriceMode>('total');
  let eFees     = $state<number | null>(null);
  let eDate     = $state('');
  let eSaving   = $state(false);

  const canSaveEdit = $derived(
    eQty >= 1 && ePaid != null && ePaid >= 0 && (eFees == null || eFees >= 0) && !!eDate && !eSaving,
  );

  function startEdit(p: Purchase) {
    editingId = p.id;
    eQty  = p.quantity;
    ePaid = p.totalPrice;
    eMode = 'total';
    eFees = p.fees || null;
    eDate = p.purchasedOn;
  }

  async function saveEdit(e: SubmitEvent) {
    e.preventDefault();
    if (editingId == null || !canSaveEdit) return;
    eSaving = true;
    try {
      const res = await fetch(`/api/purchases/${editingId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quantity: eQty, ...priceFields(eMode, ePaid!), fees: eFees ?? 0, purchasedOn: eDate }),
      });
      if (!res.ok) {
        toast.show(`Failed to save: ${(await res.text()).trim() || res.status}`, 'error');
        return;
      }
      const updated: Purchase = await res.json();
      purchases = purchases.map(x => (x.id === updated.id ? updated : x)).sort(byNewest);
      editingId = null;
    } catch {
      toast.show('Failed to save purchase', 'error');
    } finally {
      eSaving = false;
    }
  }

  function onDelete(p: Purchase) {
    toast.confirm(`Delete the ${p.purchasedOn} purchase of ${p.setName} ${p.productType}? Stock is not changed.`, async () => {
      const res = await fetch(`/api/purchases/${p.id}`, { method: 'DELETE' });
      if (!res.ok) {
        toast.show(`Failed to delete: ${res.status} ${res.statusText}`, 'error');
        return;
      }
      purchases = purchases.filter(x => x.id !== p.id);
    });
  }

  const money = (n: number | null | undefined) => (n != null ? `$${n.toFixed(2)}` : '—');
  const signed = (n: number) => `${n >= 0 ? '+' : '−'}$${Math.abs(n).toFixed(2)}`;
  const tone = (n: number | null) => (n == null ? 'none' : n > 0.005 ? 'good' : n < -0.005 ? 'bad' : 'fair');
</script>

<!-- ── Overview ───────────────────────────────────────────── -->
<div class="summary">
  <div class="card">
    <span class="card__label">Invested</span>
    <span class="card__value">{money(sum.paid)}</span>
    <span class="card__sub">
      {sum.packs} pack{sum.packs === 1 ? '' : 's'}{#if sum.fees > 0}&nbsp;· incl. {money(sum.fees)} tax & shipping{/if}
    </span>
  </div>
  <div class="card">
    <span class="card__label">Worth now</span>
    <span class="card__value">{money(sum.valueNow)}</span>
    <span class="card__sub">at today's market price</span>
  </div>
  <div class="card">
    <span class="card__label">Gain</span>
    <span class="card__value tone--{tone(gain)}">
      {gain != null ? signed(gain) : '—'}
      {#if gainPct != null}<span class="card__pct">{formatReturn(gainPct)}</span>{/if}
    </span>
    <span class="card__sub">
      vs pack prices{#if netGain != null && sum.fees > 0}&nbsp;· {signed(netGain)} after tax & shipping{/if}
    </span>
  </div>
</div>

<!-- ── Holdings ───────────────────────────────────────────── -->
{#if positions.length > 0}
  <section class="section">
    <h2 class="section__title">Holdings</h2>
    <div class="table-wrap">
      <table class="holdings">
        <thead>
          <tr>
            <th>Pack</th>
            <th class="num">Bought</th>
            <th class="num">Avg cost</th>
            <th class="num">Market</th>
            <th class="num">Value</th>
            <th class="num">Return</th>
          </tr>
        </thead>
        <tbody>
          {#each positions as h (h.packId)}
            <tr>
              <td class="holdings__name">{h.setName} <span class="muted">{h.productType}</span></td>
              <td class="num">{h.packs}</td>
              <td class="num">{money(h.avgCost)}</td>
              <td class="num">{money(h.currentPrice)}</td>
              <td class="num">{money(h.value)}</td>
              <td class="num tone--{tone(h.ret)}">{formatReturn(h.ret)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <p class="note">
      Costs leave out tax & shipping, since market prices don't include them. Counts are packs bought,
      including any since opened in drafts.
    </p>
  </section>
{/if}

<!-- ── Purchases ──────────────────────────────────────────── -->
<section class="section">
  <div class="section__header">
    <h2 class="section__title">Purchases</h2>
    {#if canEdit}
      <button class="btn-link" onclick={() => (formOpen = !formOpen)}>
        {formOpen ? 'Close' : '+ Log a purchase'}
      </button>
    {/if}
  </div>

  {#if formOpen && canEdit}
    <form class="form" onsubmit={submit}>
      <label class="field field--wide">
        <span>Pack</span>
        <select bind:value={packId} required>
          <option value="" disabled>Choose a pack…</option>
          {#each packOptions as p (p.id)}
            <option value={p.id}>{p.setName} — {p.productType}{loggedIds.has(p.id) ? '' : ' · no price yet'}</option>
          {/each}
        </select>
      </label>
      <label class="field">
        <span>Qty</span>
        <input type="number" min="1" max="999" bind:value={quantity} required />
      </label>
      <PriceInput bind:value={paid} bind:mode={priceMode} {quantity} />
      <label class="field">
        <span>Tax & shipping</span>
        <input type="number" min="0" step="0.01" inputmode="decimal" placeholder="incl. above" bind:value={fees} />
      </label>
      <label class="field">
        <span>Date</span>
        <input type="date" max={localToday()} bind:value={date} required />
      </label>
      <label class="check">
        <input type="checkbox" bind:checked={addToStock} />
        Also add to stock
      </label>
      <button class="btn-save" type="submit" disabled={!canSubmit}>{saving ? 'Saving…' : 'Log'}</button>
      <p class="form__hint">
        Going through old receipts? Leave "add to stock" off, since those packs are already counted.
        The date stays set between entries.
        {#if unloggedCount > 0}<strong>{unloggedCount} pack type{unloggedCount === 1 ? '' : 's'} with no price yet.</strong>{/if}
      </p>
    </form>
  {/if}

  {#if purchases.length === 0}
    <p class="empty">
      No purchases logged yet. Enter what you paid when adding packs on Manage Collection,
      or log past purchases here.
    </p>
  {:else}
    <ul class="log">
      {#each purchases as p (p.id)}
        {@const each = packCost(p) / p.quantity}
        {@const d = discount(each, p.currentPrice)}
        <li class="log__item">
          <div class="log__main">
            <span class="log__date">{p.purchasedOn}</span>
            <span class="log__name">{p.setName} <span class="muted">{p.productType}</span></span>
            {#if canEdit && editingId !== p.id}
              <button class="btn-row" title="Edit purchase" onclick={() => startEdit(p)}>Edit</button>
              <button class="btn-row btn-row--danger" title="Delete purchase" onclick={() => onDelete(p)}>✕</button>
            {/if}
          </div>
          {#if editingId === p.id}
            <form class="form form--inline" onsubmit={saveEdit}>
              <label class="field">
                <span>Qty</span>
                <input type="number" min="1" max="999" bind:value={eQty} required />
              </label>
              <PriceInput bind:value={ePaid} bind:mode={eMode} quantity={eQty} />
              <label class="field">
                <span>Tax & shipping</span>
                <input type="number" min="0" step="0.01" inputmode="decimal" placeholder="incl. above" bind:value={eFees} />
              </label>
              <label class="field">
                <span>Date</span>
                <input type="date" max={localToday()} bind:value={eDate} required />
              </label>
              <div class="form__actions">
                <button class="btn-save" type="submit" disabled={!canSaveEdit}>{eSaving ? 'Saving…' : 'Save'}</button>
                <button class="btn-row" type="button" onclick={() => (editingId = null)}>Cancel</button>
              </div>
            </form>
          {/if}
          <div class="log__nums">
            <span>
              <strong>{money(p.totalPrice)}</strong> for {p.quantity} pack{p.quantity === 1 ? '' : 's'}{#if p.quantity > 1}&nbsp;({money(each)} each){/if}{#if p.fees > 0}&nbsp;· incl. {money(p.fees)} tax & shipping{/if}
            </span>
            <span title="Market price per pack today">
              market {money(p.currentPrice)}
              {#if d != null}<span class="tone--{verdict(d) ?? 'none'} strong">{formatDiscount(d)}</span>{/if}
            </span>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .muted { color: var(--color-text-muted); font-size: 0.85em; }
  .strong { font-weight: 600; }
  .tone--good { color: var(--color-success); }
  .tone--bad  { color: var(--color-danger); }
  .tone--fair, .tone--none { color: inherit; }

  /* ── Summary ──────────────────────────────────────────────── */
  .summary {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
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
  .card__value { font-size: 1.35rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .card__pct { font-size: 0.85rem; font-weight: 500; margin-left: 0.3rem; }
  .card__sub { font-size: 0.72rem; color: var(--color-text-muted); }

  @media (max-width: 559px) {
    .summary { grid-template-columns: 1fr; }
  }

  /* ── Sections ─────────────────────────────────────────────── */
  .section { margin-bottom: 2rem; }
  .section__header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
  }
  .section__title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-text-muted);
    margin: 0 0 0.75rem;
  }
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
  .holdings__name { white-space: normal !important; min-width: 10rem; }

  /* ── Form ─────────────────────────────────────────────────── */
  .btn-link {
    background: none;
    border: none;
    color: var(--color-accent);
    font-size: 0.85rem;
    cursor: pointer;
    padding: 0;
  }
  .form {
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
  .form--inline { padding: 0.5rem 0 0.25rem; margin: 0; border: none; background: none; }
  .form__actions { display: flex; align-items: center; gap: 0.5rem; align-self: center; }
  .form__hint {
    flex-basis: 100%;
    margin: 0;
    font-size: 0.72rem;
    color: var(--color-text-muted);
    opacity: 0.85;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }
  .field--wide { flex: 1 1 100%; }
  .field input, .field select {
    padding: 0.35rem 0.5rem;
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }
  .field input[type='number'] { width: 6.5rem; }
  .field input:focus, .field select:focus { outline: none; border-color: var(--color-accent); }
  .check {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.82rem;
    color: var(--color-text-muted);
    align-self: center;
  }
  .check input { accent-color: var(--color-accent); }
  .btn-save {
    background: var(--color-accent);
    color: white;
    border: none;
    border-radius: var(--radius);
    padding: 0.45rem 1.1rem;
    font-size: 0.9rem;
    cursor: pointer;
    align-self: center;
  }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }

  /* ── Log ──────────────────────────────────────────────────── */
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
  .btn-row {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: var(--radius);
    padding: 0.1rem 0.45rem;
    font-size: 0.72rem;
    cursor: pointer;
    flex-shrink: 0;
  }
  .btn-row:hover { border-color: var(--color-accent); color: var(--color-accent); }
  .btn-row--danger:hover { border-color: var(--color-danger); color: var(--color-danger); }
</style>

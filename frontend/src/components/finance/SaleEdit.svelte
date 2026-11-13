<script lang="ts">
  import { untrack } from 'svelte';
  import PriceInput from './PriceInput.svelte';
  import { send } from '../../lib/send';
  import { localToday, type PriceMode } from '../../lib/purchases';
  import type { Disposal } from '../../lib/portfolio';

  let { sale, onsaved, oncancel }: {
    sale: Disposal;
    onsaved: (d: Disposal) => void;
    oncancel: () => void;
  } = $props();

  const start = untrack(() => ({ ...sale }));
  let qty    = $state(start.quantity);
  let price  = $state<number | null>(start.proceeds);
  let mode   = $state<PriceMode>('total');
  let fees   = $state<number | null>(start.fees || null);
  let date   = $state(start.disposedOn);
  let saving = $state(false);

  const ok = $derived(qty >= 1 && price != null && price >= 0 && (fees == null || fees >= 0) && !!date && !saving);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const updated = await send<Disposal>(`/api/sales/${start.id}`, 'PUT', {
      quantity: qty, ...(mode === 'total' ? { saleTotal: price } : { salePrice: price }),
      fees: fees ?? 0, soldOn: date,
    }, 'Failed to save');
    saving = false;
    if (updated && updated !== true) onsaved(updated);
  }
</script>

<form class="form form--inline" onsubmit={save}>
  <label class="field">
    <span>Qty</span>
    <input type="number" min="1" max="999" bind:value={qty} required />
  </label>
  <PriceInput bind:value={price} bind:mode quantity={qty} label="Sold for" />
  <label class="field">
    <span>Fees & shipping</span>
    <input type="number" min="0" step="0.01" inputmode="decimal" bind:value={fees} />
  </label>
  <label class="field">
    <span>Date</span>
    <input type="date" max={localToday()} bind:value={date} required />
  </label>
  <div class="form__actions">
    <button class="btn-save" type="submit" disabled={!ok}>{saving ? 'Saving…' : 'Save'}</button>
    <button class="btn-row" type="button" onclick={oncancel}>Cancel</button>
  </div>
</form>

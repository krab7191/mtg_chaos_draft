<script lang="ts">
  import { untrack } from 'svelte';
  import PriceInput from './PriceInput.svelte';
  import { send } from '../../lib/send';
  import { localToday, priceFields, type Purchase, type PriceMode } from '../../lib/purchases';

  let { purchase, onsaved, oncancel }: {
    purchase: Purchase;
    onsaved: (p: Purchase) => void;
    oncancel: () => void;
  } = $props();

  // Start from the saved values; edits stay local until saved.
  const start = untrack(() => ({ ...purchase }));
  let qty    = $state(start.quantity);
  let paid   = $state<number | null>(start.totalPrice);
  let mode   = $state<PriceMode>('total');
  let fees   = $state<number | null>(start.fees || null);
  let date   = $state(start.purchasedOn);
  let saving = $state(false);

  const ok = $derived(qty >= 1 && paid != null && paid >= 0 && (fees == null || fees >= 0) && !!date && !saving);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const updated = await send<Purchase>(`/api/purchases/${start.id}`, 'PUT', {
      quantity: qty, ...priceFields(mode, paid!), fees: fees ?? 0, purchasedOn: date,
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
  <PriceInput bind:value={paid} bind:mode quantity={qty} />
  <label class="field">
    <span>Tax & shipping</span>
    <input type="number" min="0" step="0.01" inputmode="decimal" placeholder="incl. above" bind:value={fees} />
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

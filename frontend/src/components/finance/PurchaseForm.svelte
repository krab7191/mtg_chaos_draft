<script lang="ts">
  import PriceInput from './PriceInput.svelte';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { localToday, priceFields, type Purchase, type PriceMode } from '../../lib/purchases';

  interface PackOption { id: number; setName: string; productType: string }

  let { packs, loggedIds, oncreated }: {
    packs: PackOption[];
    loggedIds: Set<number>;
    oncreated: (p: Purchase, addedToStock: boolean) => void;
  } = $props();

  let packId     = $state<number | ''>('');
  let quantity   = $state(1);
  let paid       = $state<number | null>(null);
  let mode       = $state<PriceMode>('total');
  let fees       = $state<number | null>(null);
  let date       = $state(localToday());
  let addToStock = $state(false);
  let saving     = $state(false);

  const unlogged = $derived(packs.filter(p => !loggedIds.has(p.id)).length);
  const ok = $derived(packId !== '' && quantity >= 1 && paid != null && paid >= 0 && (fees == null || fees >= 0) && !!date && !saving);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const created = await send<Purchase>('/api/purchases', 'POST', {
      packId, quantity, ...priceFields(mode, paid!), fees: fees ?? 0, purchasedOn: date, addToStock,
    }, 'Failed to log purchase');
    saving = false;
    if (!created || created === true) return;
    oncreated(created, addToStock);
    toast.show('Purchase logged', 'success');
    // Keep the date for the next line of the same receipt.
    packId = '';
    paid = null;
    fees = null;
    quantity = 1;
  }
</script>

<form class="form" onsubmit={submit}>
  <label class="field field--wide">
    <span>Pack</span>
    <select bind:value={packId} required>
      <option value="" disabled>Choose a pack…</option>
      {#each packs as p (p.id)}
        <option value={p.id}>{p.setName} — {p.productType}{loggedIds.has(p.id) ? '' : ' · no price yet'}</option>
      {/each}
    </select>
  </label>
  <label class="field">
    <span>Qty</span>
    <input type="number" min="1" max="999" bind:value={quantity} required />
  </label>
  <PriceInput bind:value={paid} bind:mode {quantity} />
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
  <button class="btn-save" type="submit" disabled={!ok}>{saving ? 'Saving…' : 'Log purchase'}</button>
  <p class="form__hint">
    Going through old receipts? Leave "add to stock" off, since those packs are already counted.
    The date stays set between entries.
    {#if unlogged > 0}<strong>{unlogged} pack type{unlogged === 1 ? '' : 's'} with no price yet.</strong>{/if}
  </p>
</form>

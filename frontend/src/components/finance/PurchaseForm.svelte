<script lang="ts">
  import PriceInput from './PriceInput.svelte';
  import PackPicker, { type PackChoice, type PickerPack } from './PackPicker.svelte';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { localToday, priceFields, type Purchase, type PriceMode } from '../../lib/purchases';

  /** A pack that was just created in the collection (picked from MTGStocks). */
  interface NewPack extends PickerPack { quantity: number; marketPrice: number | null }

  let { packs, loggedIds, oncreated, onpackadded }: {
    packs: PickerPack[];
    loggedIds: Set<number>;
    oncreated: (p: Purchase, addedToStock: boolean) => void;
    onpackadded: (pack: NewPack) => void;
  } = $props();

  let choice     = $state<PackChoice | null>(null);
  let quantity   = $state(1);
  let paid       = $state<number | null>(null);
  let mode       = $state<PriceMode>('total');
  let fees       = $state<number | null>(null);
  let date       = $state(localToday());
  let addToStock = $state(false);
  let saving     = $state(false);

  const unlogged = $derived(packs.filter(p => !loggedIds.has(p.id)).length);
  const ok = $derived(choice != null && quantity >= 1 && paid != null && paid >= 0 && (fees == null || fees >= 0) && !!date && !saving);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const purchase = { quantity, ...priceFields(mode, paid!), fees: fees ?? 0, purchasedOn: date, addToStock };
    let created: Purchase | null = null;
    if (choice!.kind === 'pack') {
      const res = await send<Purchase>('/api/purchases', 'POST', { packId: choice!.pack.id, ...purchase }, 'Failed to log purchase');
      if (res && res !== true) created = res;
    } else {
      // Not in the collection yet: add the pack (no stock unless ticked) and the purchase together.
      const c = choice!.product;
      const res = await send<NewPack & { purchase?: Purchase }>('/api/collection', 'POST', {
        mtgstocksId: c.mtgstocksId, name: c.name, setName: c.setName, setCode: c.setCode,
        productType: c.productType, marketPrice: c.marketPrice, weight: 1, ...purchase,
      }, 'Failed to add pack');
      if (res && res !== true) {
        const { purchase: bought, ...pack } = res;
        onpackadded(pack);
        created = bought ?? null;
      }
    }
    saving = false;
    if (!created) return;
    oncreated(created, addToStock);
    toast.show('Purchase logged', 'success');
    // Keep the date for the next line of the same receipt.
    choice = null;
    paid = null;
    fees = null;
    quantity = 1;
  }
</script>

<form class="form" onsubmit={submit}>
  <PackPicker {packs} bind:value={choice} searchCatalog
    describe={p => (loggedIds.has(p.id) ? '' : ' · no price yet')} />
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

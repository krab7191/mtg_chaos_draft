<script lang="ts">
  import PriceInput from './PriceInput.svelte';
  import PackPicker, { type PackChoice } from './PackPicker.svelte';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { localToday, type PriceMode } from '../../lib/purchases';
  import type { Disposal } from '../../lib/portfolio';

  interface PackOption { id: number; setName: string; productType: string; quantity: number }

  let { packs, oncreated }: {
    packs: PackOption[];
    oncreated: (d: Disposal) => void;
  } = $props();

  const inStock = $derived(packs.filter(p => p.quantity > 0));

  let choice   = $state<PackChoice | null>(null);
  const packId = $derived(choice?.kind === 'pack' ? choice.pack.id : null);
  let quantity = $state(1);
  let price    = $state<number | null>(null);
  let mode     = $state<PriceMode>('total');
  let fees     = $state<number | null>(null);
  let date     = $state(localToday());
  let saving   = $state(false);

  const stock = $derived(packs.find(p => p.id === packId)?.quantity ?? 0);
  const ok = $derived(
    packId != null && quantity >= 1 && quantity <= stock && price != null && price >= 0 &&
    (fees == null || fees >= 0) && !!date && !saving,
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const created = await send<Disposal>('/api/sales', 'POST', {
      packId, quantity, ...(mode === 'total' ? { saleTotal: price } : { salePrice: price }),
      fees: fees ?? 0, soldOn: date,
    }, 'Failed to record sale');
    saving = false;
    if (!created || created === true) return;
    oncreated(created);
    toast.show('Sale recorded', 'success');
    choice = null;
    price = null;
    fees = null;
    quantity = 1;
  }
</script>

<form class="form" onsubmit={submit}>
  <PackPicker packs={inStock} bind:value={choice}
    describe={p => ` (${inStock.find(x => x.id === p.id)?.quantity ?? 0} in stock)`} />
  <label class="field">
    <span>Qty</span>
    <input type="number" min="1" max={stock || undefined} bind:value={quantity} required />
  </label>
  <PriceInput bind:value={price} bind:mode {quantity} label="Sold for" />
  <label class="field">
    <span>Fees & shipping</span>
    <input type="number" min="0" step="0.01" inputmode="decimal" placeholder="paid by you" bind:value={fees} />
  </label>
  <label class="field">
    <span>Date</span>
    <input type="date" max={localToday()} bind:value={date} required />
  </label>
  <button class="btn-save" type="submit" disabled={!ok}>{saving ? 'Saving…' : 'Record sale'}</button>
  <p class="form__hint">
    The packs come out of stock. Fees are what the sale cost you (marketplace fees, postage).
    {#if packId != null && quantity > stock}<strong>Only {stock} in stock.</strong>{/if}
  </p>
</form>

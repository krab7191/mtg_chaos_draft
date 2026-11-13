<script lang="ts">
  import type { PriceMode } from '../../lib/purchases';

  // A price that can be typed as the total paid (a whole display at once) or
  // per pack; shows the other figure underneath so it's easy to check.
  let { value = $bindable(null), mode = $bindable('total'), quantity, label = 'Paid' }: {
    value?: number | null;
    mode?: PriceMode;
    quantity: number;
    label?: string;
  } = $props();

  const other = $derived.by(() => {
    if (value == null || !(quantity >= 1)) return '';
    if (mode === 'total') return quantity > 1 ? `= $${(value / quantity).toFixed(2)} each` : '';
    return `= $${(value * quantity).toFixed(2)} total`;
  });
</script>

<div class="price">
  <div class="price__top">
    <span class="price__label">{label}</span>
    <div class="price__modes" role="group" aria-label="Price is">
      <button type="button" class="price__mode" class:price__mode--on={mode === 'total'}
        aria-pressed={mode === 'total'} onclick={() => (mode = 'total')}>Total</button>
      <button type="button" class="price__mode" class:price__mode--on={mode === 'each'}
        aria-pressed={mode === 'each'} onclick={() => (mode = 'each')}>Each</button>
    </div>
  </div>
  <input
    class="price__input"
    type="number"
    min="0"
    step="0.01"
    inputmode="decimal"
    placeholder={mode === 'total' ? '$ total' : '$ per pack'}
    aria-label={mode === 'total' ? `${label}, total` : `${label}, per pack`}
    bind:value
    required
  />
  <span class="price__other">{other}</span>
</div>

<style>
  .price {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }
  .price__top { display: flex; align-items: center; justify-content: space-between; gap: 0.4rem; }
  .price__modes { display: flex; }
  .price__mode {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    font-size: 0.68rem;
    padding: 0.05rem 0.4rem;
    cursor: pointer;
  }
  .price__mode:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .price__mode:last-child  { border-radius: 0 var(--radius) var(--radius) 0; border-left: none; }
  .price__mode--on { border-color: var(--color-accent); color: var(--color-accent); }
  .price__mode--on + .price__mode { border-left: 1px solid var(--color-accent); }
  .price__input {
    width: 7.5rem;
    padding: 0.35rem 0.5rem;
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }
  .price__input:focus { outline: none; border-color: var(--color-accent); }
  .price__other { min-height: 1em; font-size: 0.7rem; font-variant-numeric: tabular-nums; }
</style>

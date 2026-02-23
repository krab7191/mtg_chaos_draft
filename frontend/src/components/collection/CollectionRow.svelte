<script lang="ts">
  interface Pack {
    id: number;
    productType: string;
    marketPrice: number | null;
    quantity: number;
  }

  let { pack, onQtyChange, onDelete }: {
    pack: Pack;
    onQtyChange: (id: number, delta: number) => void;
    onDelete: (id: number, label: string) => void;
  } = $props();
</script>

<div class="pack-row" class:row--empty={pack.quantity === 0}>
  <span class="pack-row__type">{pack.productType}</span>
  <div class="qty">
    <button
      class="qty__btn qty__btn--dec"
      onclick={() => onQtyChange(pack.id, -1)}
      disabled={pack.quantity === 0}
    >−</button>
    <span class="qty__val">{pack.quantity}</span>
    <button
      class="qty__btn qty__btn--inc"
      onclick={() => onQtyChange(pack.id, +1)}
    >+</button>
  </div>
  <span class="price-val">
    {pack.marketPrice != null ? `$${pack.marketPrice.toFixed(2)}` : '—'}
  </span>
  <button
    class="btn-delete"
    onclick={() => onDelete(pack.id, pack.productType)}
    title="Remove"
  >✕</button>
</div>

<style>
  .pack-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid var(--color-border);
  }
  .pack-row:last-child { border-bottom: none; }
  .pack-row:hover { background: var(--color-surface); }
  .pack-row.row--empty { opacity: 0.4; }

  .pack-row__type {
    flex: 1;
    min-width: 0;
    font-size: 0.9rem;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .price-val {
    font-size: 0.85rem;
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
    width: 4.5rem;
    text-align: right;
    flex-shrink: 0;
  }

  .qty {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-shrink: 0;
  }

  .qty__val {
    min-width: 1.5rem;
    text-align: center;
    font-variant-numeric: tabular-nums;
    font-size: 0.9rem;
  }

  .qty__btn {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: var(--radius);
    width: 1.6rem;
    height: 1.6rem;
    font-size: 1rem;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    cursor: pointer;
  }
  .qty__btn:hover { border-color: var(--color-accent); color: var(--color-accent); }
  .qty__btn:disabled { opacity: 0.3; cursor: not-allowed; }

  .btn-delete {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: var(--radius);
    padding: 0.2rem 0.45rem;
    font-size: 0.78rem;
    flex-shrink: 0;
    cursor: pointer;
  }
  .btn-delete:hover { border-color: var(--color-danger); color: var(--color-danger); }
</style>

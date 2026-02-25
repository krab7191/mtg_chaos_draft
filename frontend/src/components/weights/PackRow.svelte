<script lang="ts">
  interface Pack {
    id: number;
    productType: string;
    marketPrice: number | null;
    quantity: number;
    cardsPerPack: number;
  }

  let { pack, odds, multiplier, onMultChange }: {
    pack: Pack;
    odds: number;
    multiplier: number;
    onMultChange: (id: string, value: number) => void;
  } = $props();

  const fmt = (n: number) => n > 0 ? `+${n.toFixed(1)}` : n.toFixed(1);
  const slots = Math.ceil(15 / Math.max(1, pack.cardsPerPack ?? 15));
  const effectiveSlots = Math.floor(pack.quantity / slots);
</script>

<div class="pack-row">
  <div class="pack-row__type">{pack.productType}</div>
  <div class="pack-row__stats">
    <span
      class="pack-row__qty"
      title={pack.cardsPerPack < 12 ? `${pack.cardsPerPack} cards/pack, ${slots}× per slot` : undefined}
    >{effectiveSlots}{pack.cardsPerPack < 12 ? '*' : ''} slots</span>
    <span class="pack-row__price">
      {pack.marketPrice != null ? `$${pack.marketPrice.toFixed(2)}` : '—'}
    </span>
    <div class="odds">
      <div class="odds__bar">
        <div class="odds__fill" style="width:{odds.toFixed(1)}%"></div>
      </div>
      <span class="odds__label">{odds.toFixed(1)}%</span>
    </div>
    <div class="weight-ctrl">
      <button class="weight-btn" onclick={() => onMultChange(String(pack.id), Math.round((multiplier - 0.1) * 10) / 10)}>-</button>
      <span class="weight-val" class:pos={multiplier > 0} class:neg={multiplier < 0}>
        {fmt(multiplier)}
      </span>
      <button class="weight-btn" onclick={() => onMultChange(String(pack.id), Math.round((multiplier + 0.1) * 10) / 10)}>+</button>
    </div>
  </div>
</div>

<style>
  .pack-row {
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid var(--color-border);
  }
  .pack-row:last-child { border-bottom: none; }

  .pack-row__type {
    font-size: 0.9rem;
    font-weight: 400;
    color: var(--color-text-muted);
    margin-bottom: 0.2rem;
  }

  .pack-row__stats {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .pack-row__qty,
  .pack-row__price {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }

  .odds {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .odds__bar {
    width: 60px;
    flex-shrink: 0;
    height: 5px;
    background: var(--color-border);
    border-radius: 3px;
    overflow: hidden;
  }

  .odds__fill {
    height: 100%;
    background: var(--color-accent);
    border-radius: 3px;
    transition: width 0.25s ease;
  }

  .odds__label {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    width: 3rem;
    text-align: right;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }

  /* ── Weight control ───────────────────────────────────── */
  .weight-ctrl {
    display: flex;
    align-items: center;
    gap: 0.15rem;
    flex-shrink: 0;
  }

  .weight-btn {
    width: 1.75rem;
    height: 1.75rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 1rem;
    line-height: 1;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    touch-action: manipulation;
  }
  .weight-btn:hover { border-color: var(--color-accent); }
  .weight-btn:active { opacity: 0.7; }

  .weight-val {
    min-width: 2.2rem;
    text-align: center;
    font-size: 0.85rem;
    font-weight: 500;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-muted);
  }
  .weight-val.pos { color: var(--color-accent); }
  .weight-val.neg { color: var(--color-text-muted); opacity: 0.6; }
</style>

export interface Purchase {
  id: number;
  packId: number;
  setName: string;
  productType: string;
  quantity: number;
  /** What was paid for the whole purchase, tax & shipping included. */
  totalPrice: number;
  /** Tax & shipping within totalPrice. */
  fees: number;
  /** totalPrice / quantity. */
  unitPrice: number;
  /** Market price per pack today. */
  currentPrice: number | null;
  /** YYYY-MM-DD */
  purchasedOn: string;
}

/**
 * What the packs themselves cost, without tax & shipping. Market prices don't
 * include those either, so this is what gets compared with the market.
 */
export function packCost(p: Pick<Purchase, 'totalPrice' | 'fees'>): number {
  return p.totalPrice - (p.fees ?? 0);
}

/**
 * How far below the market a price is, as a fraction of the market price:
 * 0.2 = 20% under market (good), -0.1 = 10% over (bad).
 * Null when the market price isn't known.
 */
export function discount(paid: number, market: number | null | undefined): number | null {
  if (market == null || market <= 0) return null;
  return (market - paid) / market;
}

export type Verdict = 'good' | 'fair' | 'bad';

/** Within 5% of market counts as fair. */
export function verdict(d: number | null): Verdict | null {
  if (d == null) return null;
  if (d >= 0.05) return 'good';
  if (d <= -0.05) return 'bad';
  return 'fair';
}

export function formatDiscount(d: number | null): string {
  if (d == null) return '—';
  const pct = Math.round(Math.abs(d) * 100);
  if (pct === 0) return 'at market';
  return `${pct}% ${d > 0 ? 'under' : 'over'}`;
}

/** Gain as a fraction of cost: 0.25 = +25%. Null when there's no cost. */
export function returnPct(value: number, cost: number): number | null {
  return cost > 0 ? (value - cost) / cost : null;
}

export function formatReturn(r: number | null): string {
  if (r == null) return '—';
  const pct = Math.round(r * 1000) / 10;
  return `${pct > 0 ? '+' : pct < 0 ? '−' : ''}${Math.abs(pct)}%`;
}

export interface PurchaseTotals {
  packs: number;
  /** Everything spent, tax & shipping included. */
  paid: number;
  fees: number;
  /** Market value today of the packs bought, where the price is known. */
  valueNow: number | null;
  /** Pack cost (no fees) of the purchases counted in valueNow. */
  costWithValueNow: number;
  /** Full cost (with fees) of the purchases counted in valueNow. */
  paidWithValueNow: number;
}

export function totals(purchases: Purchase[]): PurchaseTotals {
  let packs = 0, paid = 0, fees = 0;
  let valueNow = 0, costWithValueNow = 0, paidWithValueNow = 0, priced = 0;
  for (const p of purchases) {
    packs += p.quantity;
    paid += p.totalPrice;
    fees += p.fees ?? 0;
    if (p.currentPrice != null) {
      valueNow += p.currentPrice * p.quantity;
      costWithValueNow += packCost(p);
      paidWithValueNow += p.totalPrice;
      priced++;
    }
  }
  return {
    packs, paid, fees,
    valueNow: priced ? valueNow : null,
    costWithValueNow,
    paidWithValueNow,
  };
}

/** Every purchase of one pack, combined like a stock position. */
export interface Holding {
  packId: number;
  setName: string;
  productType: string;
  packs: number;
  /** Pack cost without fees. */
  cost: number;
  fees: number;
  /** cost / packs */
  avgCost: number;
  currentPrice: number | null;
  value: number | null;
  /** (value - cost) / cost */
  ret: number | null;
}

/** Combine purchases per pack, biggest cost first. */
export function holdings(purchases: Purchase[]): Holding[] {
  const map = new Map<number, Holding>();
  for (const p of purchases) {
    let h = map.get(p.packId);
    if (!h) {
      h = {
        packId: p.packId, setName: p.setName, productType: p.productType,
        packs: 0, cost: 0, fees: 0, avgCost: 0,
        currentPrice: p.currentPrice, value: null, ret: null,
      };
      map.set(p.packId, h);
    }
    h.packs += p.quantity;
    h.cost += packCost(p);
    h.fees += p.fees ?? 0;
  }
  for (const h of map.values()) {
    h.avgCost = h.packs ? h.cost / h.packs : 0;
    if (h.currentPrice != null) {
      h.value = h.currentPrice * h.packs;
      h.ret = returnPct(h.value, h.cost);
    }
  }
  return [...map.values()].sort((a, b) => b.cost - a.cost);
}

/** How a typed price is meant: for the whole purchase, or per pack. */
export type PriceMode = 'total' | 'each';

/** The API fields for a price typed in the given mode. */
export function priceFields(mode: PriceMode, value: number): { paidTotal: number } | { paidPrice: number } {
  return mode === 'total' ? { paidTotal: value } : { paidPrice: value };
}

/** Newest purchase date first; newer entries first within a day. */
export function byNewest(a: Purchase, b: Purchase): number {
  return b.purchasedOn.localeCompare(a.purchasedOn) || b.id - a.id;
}

/** Today's date in the viewer's timezone, as YYYY-MM-DD. */
export function localToday(now = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

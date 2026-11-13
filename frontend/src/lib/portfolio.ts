import { packCost, returnPct, type Purchase } from './purchases';

/** Packs that left the collection: opened in an approved draft, or sold. */
export interface Disposal {
  id: number;
  packId: number;
  setName: string;
  productType: string;
  kind: 'draft' | 'sale';
  draftId: number | null;
  quantity: number;
  /** Money in for all the packs (what was collected from draft players, or the sale price). */
  proceeds: number;
  /** Drafts: what the packs were worth at market. */
  marketValue: number | null;
  /** Selling fees & shipping paid out of the proceeds. */
  fees: number;
  /** YYYY-MM-DD */
  disposedOn: string;
}

/** What a draft or sale made, against the average cost of the packs it used. */
export interface DisposalResult {
  /** Packs with a known cost (covered by logged purchases). */
  basisPacks: number;
  /** Packs with no logged purchase to take a cost from. */
  unknownPacks: number;
  /** Cost of the basis packs. */
  cost: number;
  /** Net proceeds for the basis packs minus their cost; null when no pack had a cost. */
  gain: number | null;
}

/** One pack type, like a stock position. Costs leave out tax & shipping. */
export interface Position {
  packId: number;
  setName: string;
  productType: string;
  bought: number;
  /** Packs gone through drafts and sales. */
  disposed: number;
  /** Logged packs still held. */
  held: number;
  /** Cost of the packs still held. */
  cost: number;
  avgCost: number;
  currentPrice: number | null;
  value: number | null;
  unrealized: number | null;
  unrealizedPct: number | null;
  realized: number;
  /** Tax & shipping paid on purchases of this pack. */
  purchaseFees: number;
}

export interface Portfolio {
  positions: Position[];
  results: Map<number, DisposalResult>;
  /** Everything spent on purchases, tax & shipping included. */
  invested: number;
  purchaseFees: number;
  heldPacks: number;
  /** Cost of the packs still held. */
  cost: number;
  /** Value of the packs still held, where the market price is known. */
  value: number;
  /** Cost of the held packs that have a market price (what value compares to). */
  pricedCost: number;
  unrealized: number;
  unrealizedPct: number | null;
  realized: number;
  /** Money in from drafts and sales, after selling fees. */
  proceeds: number;
  /** Packs that left with no logged purchase behind them. */
  unknownPacks: number;
}

type Event =
  | { kind: 'buy'; date: string; id: number; p: Purchase }
  | { kind: 'out'; date: string; id: number; d: Disposal };

/**
 * Work out positions and gains with the average-cost method, replaying each
 * pack's purchases and disposals in date order (purchases first on the same
 * day). Packs leaving use the average cost of the logged packs held at the
 * time; any beyond those have no known cost and are left out of gains.
 */
export function portfolio(purchases: Purchase[], disposals: Disposal[]): Portfolio {
  const events = new Map<number, Event[]>();
  const add = (packId: number, e: Event) => {
    const list = events.get(packId);
    if (list) list.push(e); else events.set(packId, [e]);
  };
  for (const p of purchases) add(p.packId, { kind: 'buy', date: p.purchasedOn, id: p.id, p });
  for (const d of disposals) add(d.packId, { kind: 'out', date: d.disposedOn, id: d.id, d });

  const positions: Position[] = [];
  const results = new Map<number, DisposalResult>();

  for (const [packId, list] of events) {
    list.sort((a, b) =>
      a.date.localeCompare(b.date) || (a.kind === b.kind ? a.id - b.id : a.kind === 'buy' ? -1 : 1));

    const first = list[0].kind === 'buy' ? list[0].p : list[0].d;
    const pos: Position = {
      packId, setName: first.setName, productType: first.productType,
      bought: 0, disposed: 0, held: 0, cost: 0, avgCost: 0,
      currentPrice: null, value: null, unrealized: null, unrealizedPct: null,
      realized: 0, purchaseFees: 0,
    };

    for (const e of list) {
      if (e.kind === 'buy') {
        pos.bought += e.p.quantity;
        pos.held += e.p.quantity;
        pos.cost += packCost(e.p);
        pos.purchaseFees += e.p.fees ?? 0;
        pos.currentPrice = e.p.currentPrice;
        continue;
      }
      const d = e.d;
      pos.disposed += d.quantity;
      const basisPacks = Math.min(d.quantity, pos.held);
      const cost = basisPacks > 0 ? (pos.cost * basisPacks) / pos.held : 0;
      pos.held -= basisPacks;
      pos.cost -= cost;
      if (pos.held === 0) pos.cost = 0; // no rounding dust on a closed position
      const net = d.proceeds - (d.fees ?? 0);
      const gain = basisPacks > 0 ? (net * basisPacks) / d.quantity - cost : null;
      if (gain != null) pos.realized += gain;
      results.set(d.id, { basisPacks, unknownPacks: d.quantity - basisPacks, cost, gain });
    }

    pos.avgCost = pos.held > 0 ? pos.cost / pos.held : 0;
    if (pos.held > 0 && pos.currentPrice != null) {
      pos.value = pos.currentPrice * pos.held;
      pos.unrealized = pos.value - pos.cost;
      pos.unrealizedPct = returnPct(pos.value, pos.cost);
    }
    positions.push(pos);
  }

  // Open positions by cost, then closed ones by realized gain.
  positions.sort((a, b) => (b.held > 0 ? 1 : 0) - (a.held > 0 ? 1 : 0) || b.cost - a.cost || b.realized - a.realized);

  let value = 0, pricedCost = 0, cost = 0, heldPacks = 0, realized = 0;
  for (const p of positions) {
    heldPacks += p.held;
    cost += p.cost;
    realized += p.realized;
    if (p.value != null) { value += p.value; pricedCost += p.cost; }
  }
  let unknownPacks = 0;
  for (const r of results.values()) unknownPacks += r.unknownPacks;

  return {
    positions,
    results,
    invested: purchases.reduce((s, p) => s + p.totalPrice, 0),
    purchaseFees: purchases.reduce((s, p) => s + (p.fees ?? 0), 0),
    heldPacks,
    cost,
    value,
    pricedCost,
    unrealized: value - pricedCost,
    unrealizedPct: returnPct(value, pricedCost),
    realized,
    proceeds: disposals.reduce((s, d) => s + d.proceeds - (d.fees ?? 0), 0),
    unknownPacks,
  };
}

/** One line of the activity history. A draft's packs share one line. */
export type Activity =
  | { type: 'buy'; key: string; date: string; id: number; purchase: Purchase }
  | { type: 'sale'; key: string; date: string; id: number; disposal: Disposal }
  | { type: 'draft'; key: string; date: string; id: number; draftId: number | null; disposals: Disposal[] };

/** Purchases, drafts and sales together, newest first. */
export function activity(purchases: Purchase[], disposals: Disposal[]): Activity[] {
  const rows: Activity[] = purchases.map(p => ({
    type: 'buy' as const, key: `b${p.id}`, date: p.purchasedOn, id: p.id, purchase: p,
  }));
  const drafts = new Map<string, Disposal[]>();
  for (const d of disposals) {
    if (d.kind === 'sale') {
      rows.push({ type: 'sale', key: `s${d.id}`, date: d.disposedOn, id: d.id, disposal: d });
      continue;
    }
    // Records of a deleted draft can't be grouped any more: one line each.
    const key = d.draftId != null ? `d${d.draftId}` : `x${d.id}`;
    const list = drafts.get(key);
    if (list) list.push(d); else drafts.set(key, [d]);
  }
  for (const [key, list] of drafts) {
    rows.push({ type: 'draft', key, date: list[0].disposedOn, id: list[0].id, draftId: list[0].draftId, disposals: list });
  }
  // Newest first; on the same day, things leaving come after (above) the buys.
  const rank = (a: Activity) => (a.type === 'buy' ? 0 : 1);
  return rows.sort((a, b) => b.date.localeCompare(a.date) || rank(b) - rank(a) || b.id - a.id);
}

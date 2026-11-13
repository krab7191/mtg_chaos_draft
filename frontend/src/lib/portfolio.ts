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

/** A pack in the collection right now (what Manage Collection shows). */
export interface StockPack {
  id: number;
  setName: string;
  productType: string;
  quantity: number;
  marketPrice: number | null;
}

/**
 * One pack type, like a stock position. Stock is the truth for what's held;
 * costs come from logged purchases and leave out tax & shipping.
 */
export interface Position {
  packId: number;
  setName: string;
  productType: string;
  /** Packs logged as bought. */
  bought: number;
  /** Packs gone through recorded drafts and sales. */
  disposed: number;
  /** Packs in stock now. */
  stock: number;
  /** Packs in stock with a logged cost. */
  costed: number;
  /** Packs in stock beyond the logged ones: no known cost. */
  uncosted: number;
  /** Logged packs no longer in stock and not sold or drafted: opened or given away. */
  opened: number;
  openedCost: number;
  /** Cost of the costed packs in stock. */
  cost: number;
  avgCost: number;
  currentPrice: number | null;
  /** All packs in stock at today's market price. */
  value: number | null;
  /** Costed packs at today's market price, minus their cost. */
  unrealized: number | null;
  unrealizedPct: number | null;
  realized: number;
  /** Tax & shipping paid on purchases of this pack. */
  purchaseFees: number;
}

export interface Portfolio {
  positions: Position[];
  results: Map<number, DisposalResult>;
  /** Everything spent on logged purchases, tax & shipping included. */
  invested: number;
  purchaseFees: number;
  boughtPacks: number;
  stockPacks: number;
  uncostedPacks: number;
  /** Cost of the costed packs in stock. */
  cost: number;
  /** All stock at today's market price, where known. */
  value: number;
  /** Cost of costed packs that have a market price (what unrealized compares to). */
  pricedCost: number;
  unrealized: number;
  unrealizedPct: number | null;
  realized: number;
  /** Money in from drafts and sales, after selling fees. */
  proceeds: number;
  /** Packs sold or drafted with no logged purchase behind them. */
  unknownPacks: number;
  openedPacks: number;
  openedCost: number;
}

type Event =
  | { kind: 'buy'; date: string; id: number; p: Purchase }
  | { kind: 'out'; date: string; id: number; d: Disposal };

/**
 * Work out positions and gains with the average-cost method, replaying each
 * pack's purchases and recorded drafts/sales in date order (purchases first on
 * the same day). Packs leaving use the average cost at the time; any beyond
 * the logged ones have no known cost and are left out of gains.
 *
 * Current stock then decides what's held. Logged packs that are no longer in
 * stock were opened or given away: they leave at average cost, and aren't a
 * gain or a loss.
 */
export function portfolio(purchases: Purchase[], disposals: Disposal[], stock: StockPack[]): Portfolio {
  const events = new Map<number, Event[]>();
  const add = (packId: number, e: Event) => {
    const list = events.get(packId);
    if (list) list.push(e); else events.set(packId, [e]);
  };
  for (const p of purchases) add(p.packId, { kind: 'buy', date: p.purchasedOn, id: p.id, p });
  for (const d of disposals) add(d.packId, { kind: 'out', date: d.disposedOn, id: d.id, d });
  const stockById = new Map(stock.map(s => [s.id, s]));

  const positions: Position[] = [];
  const results = new Map<number, DisposalResult>();
  const ids = new Set([...events.keys(), ...stock.filter(s => s.quantity > 0).map(s => s.id)]);

  for (const packId of ids) {
    const list = events.get(packId) ?? [];
    list.sort((a, b) =>
      a.date.localeCompare(b.date) || (a.kind === b.kind ? a.id - b.id : a.kind === 'buy' ? -1 : 1));

    const sp = stockById.get(packId);
    const named = sp ?? (list[0].kind === 'buy' ? list[0].p : list[0].d);
    let held = 0, cost = 0, lastPrice: number | null = null;
    const pos: Position = {
      packId, setName: named.setName, productType: named.productType,
      bought: 0, disposed: 0, stock: sp?.quantity ?? 0, costed: 0, uncosted: 0, opened: 0, openedCost: 0,
      cost: 0, avgCost: 0, currentPrice: null, value: null, unrealized: null, unrealizedPct: null,
      realized: 0, purchaseFees: 0,
    };

    for (const e of list) {
      if (e.kind === 'buy') {
        pos.bought += e.p.quantity;
        pos.purchaseFees += e.p.fees ?? 0;
        held += e.p.quantity;
        cost += packCost(e.p);
        lastPrice = e.p.currentPrice;
        continue;
      }
      const d = e.d;
      pos.disposed += d.quantity;
      const basisPacks = Math.min(d.quantity, held);
      const out = basisPacks > 0 ? (cost * basisPacks) / held : 0;
      held -= basisPacks;
      cost = held > 0 ? cost - out : 0; // no rounding dust on a closed position
      const net = d.proceeds - (d.fees ?? 0);
      const gain = basisPacks > 0 ? (net * basisPacks) / d.quantity - out : null;
      if (gain != null) pos.realized += gain;
      results.set(d.id, { basisPacks, unknownPacks: d.quantity - basisPacks, cost: out, gain });
    }

    // Stock decides what's held now.
    pos.costed = Math.min(pos.stock, held);
    pos.uncosted = pos.stock - pos.costed;
    pos.opened = held - pos.costed;
    pos.openedCost = held > 0 ? (cost * pos.opened) / held : 0;
    pos.cost = cost - pos.openedCost;
    if (pos.costed === 0) pos.cost = 0;
    pos.avgCost = pos.costed > 0 ? pos.cost / pos.costed : 0;

    pos.currentPrice = sp?.marketPrice ?? lastPrice;
    if (pos.currentPrice != null) {
      if (pos.stock > 0) pos.value = pos.currentPrice * pos.stock;
      if (pos.costed > 0) {
        pos.unrealized = pos.currentPrice * pos.costed - pos.cost;
        pos.unrealizedPct = returnPct(pos.currentPrice * pos.costed, pos.cost);
      }
    }
    positions.push(pos);
  }

  // Biggest holdings first; then what's left (sold out, opened) by realized gain.
  positions.sort((a, b) => (b.value ?? 0) - (a.value ?? 0) || b.cost - a.cost || b.realized - a.realized);

  const t = {
    boughtPacks: 0, stockPacks: 0, uncostedPacks: 0, cost: 0, value: 0, pricedCost: 0,
    costedValue: 0, realized: 0, openedPacks: 0, openedCost: 0,
  };
  for (const p of positions) {
    t.boughtPacks += p.bought;
    t.stockPacks += p.stock;
    t.uncostedPacks += p.uncosted;
    t.cost += p.cost;
    t.realized += p.realized;
    t.openedPacks += p.opened;
    t.openedCost += p.openedCost;
    if (p.value != null) t.value += p.value;
    if (p.unrealized != null) {
      t.pricedCost += p.cost;
      t.costedValue += p.unrealized + p.cost;
    }
  }
  let unknownPacks = 0;
  for (const r of results.values()) unknownPacks += r.unknownPacks;

  return {
    positions,
    results,
    invested: purchases.reduce((s, p) => s + p.totalPrice, 0),
    purchaseFees: purchases.reduce((s, p) => s + (p.fees ?? 0), 0),
    boughtPacks: t.boughtPacks,
    stockPacks: t.stockPacks,
    uncostedPacks: t.uncostedPacks,
    cost: t.cost,
    value: t.value,
    pricedCost: t.pricedCost,
    unrealized: t.costedValue - t.pricedCost,
    unrealizedPct: returnPct(t.costedValue, t.pricedCost),
    realized: t.realized,
    proceeds: disposals.reduce((s, d) => s + d.proceeds - (d.fees ?? 0), 0),
    unknownPacks,
    openedPacks: t.openedPacks,
    openedCost: t.openedCost,
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

// Best deals: rank every buyable pack by what you get for your money, with
// shipping, and whether waiting would likely make it cheaper.
//
//   score = contents ÷ (price + shipping per pack) × timing
//
// timing (from the API) is 1 + the median 3-month price change for packs with
// the same signal on real histories.

import { applyFilters, type OfferFilters, type StoreOffer } from './deals';
import type { Signal } from './trends';

export interface Candidate {
  mtgstocksId: number;
  setName: string;
  productType: string;
  releasedAt: string;
  store: string;
  url: string;
  title: string;
  language: string;
  price: number;
  market: number;
  contents: number;
  signal: Signal | '';
  timing: number;
  evChange?: number;
}

export interface BestDealsResponse {
  candidates: Candidate[];
  /** Default shipping per order, by store. */
  shipping: Record<string, number>;
  asOf: string;
}

export interface ShippingSettings {
  /** Cost per order, by store. */
  perOrder: Record<string, number>;
  /** Packs usually bought per order (shipping is shared across them). */
  packsPerOrder: number;
}

export interface Ranked extends Candidate {
  /** Price plus this pack's share of shipping. */
  delivered: number;
  /** Dollars of cards per dollar delivered. */
  perDollar: number;
  score: number;
}

/** Shipping per order for a store without a setting: cautious, so an unknown
 *  store never wins on free shipping it doesn't have ($8+ is common for small shops). */
export const UNKNOWN_SHIPPING = 8;

/** Scores candidates that pass the filters, best first. */
export function rank(candidates: Candidate[], filters: OfferFilters, ship: ShippingSettings): Ranked[] {
  // Reuse the store-listing filters (language, max price); "above market" is
  // about listings, not picks, so it doesn't apply here.
  const asOffers = candidates.map(c => ({ ...c, discount: 1, available: true }) as unknown as StoreOffer);
  const keep = new Set(applyFilters(asOffers, { ...filters, showOver: true }));
  const perPack = (store: string) => (ship.perOrder[store] ?? UNKNOWN_SHIPPING) / Math.max(1, ship.packsPerOrder);
  return candidates
    .filter((_, i) => keep.has(asOffers[i]))
    .map(c => {
      const delivered = c.price + perPack(c.store);
      const perDollar = c.contents / delivered;
      return { ...c, delivered, perDollar, score: perDollar * c.timing };
    })
    .sort((a, b) => b.score - a.score);
}

/** A live CardTrader listing (from /api/best-deals/live). */
export interface LiveOffer { mtgstocksId: number; store: string; url: string; title: string; language: string; price: number }

/** Stores whose listings are checked live (the rest are links or market prices). */
export const LIVE_STORES = ['CardTrader', 'Card Kingdom'];

/** Swaps the live stores' candidates of the checked packs for their live listings. */
export function withLive(candidates: Candidate[], checked: number[], live: LiveOffer[]): Candidate[] {
  const done = new Set(checked);
  const base = new Map<number, Candidate>();
  for (const c of candidates) if (!base.has(c.mtgstocksId)) base.set(c.mtgstocksId, c);
  const kept = candidates.filter(c => !(LIVE_STORES.includes(c.store) && done.has(c.mtgstocksId)));
  const fresh = live.flatMap(o => {
    const b = base.get(o.mtgstocksId);
    return b ? [{ ...b, store: o.store, url: o.url, title: o.title, language: o.language || 'en', price: o.price }] : [];
  });
  return [...kept, ...fresh];
}

/** Packs among the best `n`, with a live store's listing, not checked live
 *  recently (fresh says whether a pack's last check still counts). */
export function toCheck(ranked: Ranked[], fresh: (id: number) => boolean, n = 10): number[] {
  const ids: number[] = [];
  for (const r of topPicks(ranked, n)) {
    const live = ranked.some(x => x.mtgstocksId === r.mtgstocksId && LIVE_STORES.includes(x.store));
    if (live && !fresh(r.mtgstocksId)) ids.push(r.mtgstocksId);
  }
  return ids;
}

/** The best `n` from different sets. */
export function topPicks(ranked: Ranked[], n = 3): Ranked[] {
  const sets = new Set<string>();
  const out: Ranked[] = [];
  for (const r of ranked) {
    if (sets.has(r.setName)) continue;
    sets.add(r.setName);
    out.push(r);
    if (out.length === n) break;
  }
  return out;
}

// The epsilon keeps float noise (1.045 − 1 = 0.04499…) from rounding down.
const pctText = (f: number) => `${Math.round(Math.abs(f) * 100 + 1e-9)}%`;

/** Why a pick is there, in plain words. */
export function reasons(r: Ranked): string[] {
  const out = [`$${r.perDollar.toFixed(2)} of cards per $1: contents $${r.contents.toFixed(2)} for $${r.delivered.toFixed(2)} delivered`];
  const under = 1 - r.price / r.market;
  if (r.store !== 'TCGplayer' && under >= 0.01) out.push(`${pctText(under)} under TCGplayer's market price ($${r.market.toFixed(2)})`);
  const move = r.timing - 1;
  if (r.signal === 'climbing') out.push(`↗ Climbing: packs like this were up ~${pctText(move)} 3 months later, so waiting rarely pays`);
  else if (r.signal === 'new') out.push(`New set: usually ~${pctText(move)} cheaper 3 months on, so it gets marked down a little`);
  else if (r.signal === 'floor') out.push('◆ Possible floor: near its low');
  else if (r.signal) out.push(`Price ${r.signal}: typical 3-month change for that is ${move >= 0 ? '+' : '−'}${pctText(move)}`);
  if (r.evChange != null && Math.abs(r.evChange) >= 0.01) {
    out.push(`Contents ${r.evChange > 0 ? '↑' : '↓'}${pctText(r.evChange)} over 2 weeks`);
  }
  return out;
}

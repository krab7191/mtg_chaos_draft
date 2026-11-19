// Types and helpers for the price trends page.
import { languageLabel } from './deals';

export type Signal = 'preorder' | 'new' | 'floor' | 'falling' | 'rising' | 'stable';

export interface Trend {
  current: number;
  low: number;
  lowDay: string;
  aboveLow: number;
  change30: number | null;
  releasePrice: number;
  dropFromRelease: number;
  band: number | null;
  /** The 7-day median price broke below its previous low (prevLow). */
  newLow?: boolean;
  prevLow?: number;
  signal: Signal;
  spark: number[];
  sparkStart: string;
  historyDays: number;
  released: string | null;
  /** Older packs: the usual price (6-month median), how far the last week is
   *  from it, and whether that's a dip (well under usual). */
  usual?: number;
  vsUsual?: number;
  dip?: boolean;
}

export interface Watch { mtgstocksId: number; dropAlert: number | null }

export type AlertReason = 'floor' | 'drop' | 'new_low' | 'deal' | 'dip';

/** The best store listing well under market (see lib/deals). */
export interface RowDeal { store: string; url: string; price: number; marketPrice: number; discount: number; language?: string }

export interface TrendRow {
  mtgstocksId: number;
  name: string;
  setName: string;
  setCode: string;
  releasedAt: string;
  productType: string;
  marketPrice: number | null;
  packsPerBox?: number;
  trend: Trend | null;
  watch: Watch | null;
  alerts: AlertReason[];
  deal?: RowDeal;
  /** Value of a pack's contents (pull rates x card prices) and its 2-week change. */
  ev?: { value: number; change?: number; day: string };
  /** Packs of this set and kind in the collection (loose packs for a box row). */
  stock?: number;
}

export interface FloorRule {
  minAge: number; drop: number; window: number; band: number; near: number;
  /** If > 0, the low must have happened within this many days. */
  recent?: number;
}

export interface Thresholds {
  newSetDays: number;
  trendWindow: number;
  trendMove: number;
  /** "regular" and "collector" floor rules. */
  floorRules: Record<string, FloorRule>;
}

/** One floor rule in words. */
export function describeRule(r: FloorRule): string {
  const pctOf = (f: number) => `${Math.round(f * 100)}%`;
  return [
    `released at least ${r.minAge} days ago`,
    ...(r.drop > 0 ? [`at least ${pctOf(r.drop)} below its release price`] : []),
    `moving within a ${pctOf(r.band)} range for the last ${r.window} days`,
    `within ${pctOf(r.near)} of its lowest price since release`,
    ...(r.recent ? [`with that low reached in the last ${r.recent} days`] : []),
  ].join(', ');
}

export interface BTMetrics {
  products: number; coverage: number; median: number; mean: number; falseRate: number;
  /** Shares of alerts within 5% / 10% of the eventual low. */
  hit5?: number; hit10?: number;
}

export interface RuleProposal {
  id: number;
  kind: 'regular' | 'collector' | string;
  rule: FloorRule;
  current: FloorRule;
  metrics: { current: BTMetrics; proposed: BTMetrics; products: number } | null;
  createdAt: string;
}

export interface TrendsResponse {
  products: TrendRow[];
  /** Floor rules the quarterly backtest suggests, awaiting a decision. */
  proposals?: RuleProposal[];
  lastBacktest?: string | null;
  /** Phone alerts (ntfy) are set up on the server. */
  notifications: boolean;
  backfilled: number;
  tracked: number;
  thresholds: Thresholds;
}

export interface PricePoint { day: string; price: number }

/** Signal labels and icons: a signal is never shown by color alone. */
export const SIGNALS: Record<Signal, { label: string; icon: string; hint: string }> = {
  floor:    { label: 'Possible floor', icon: '◆', hint: 'Fell a lot since release, then held steady near its low' },
  falling:  { label: 'Falling',        icon: '↓', hint: 'Still dropping' },
  rising:   { label: 'Rising',         icon: '↑', hint: 'Climbing' },
  stable:   { label: 'Stable',         icon: '–', hint: 'Not moving much' },
  new:      { label: 'New',            icon: '★', hint: 'Just released: hype prices' },
  preorder: { label: 'Preorder',       icon: '…', hint: 'Not released yet' },
};

export const isBox = (r: TrendRow) => (r.packsPerBox ?? 0) > 0;

/** Price per pack: a box's price split over its packs, or the pack's price. */
export function perPack(r: TrendRow): number | null {
  const price = r.trend?.current ?? r.marketPrice;
  if (price == null) return null;
  return isBox(r) ? price / r.packsPerBox! : price;
}

export const money = (n: number | null | undefined) => (n == null ? '—' : `$${n.toFixed(2)}`);

/** Signed percent, e.g. +12% or −8%; '—' when unknown. */
export function pct(f: number | null | undefined, digits = 0): string {
  if (f == null || !Number.isFinite(f)) return '—';
  const v = (f * 100).toFixed(digits);
  if (Number(v) === 0) return '0%';
  return `${f > 0 ? '+' : '−'}${v.replace('-', '')}%`;
}

/** "3 mo", "1.5 yr" — how long ago a date was. */
export function age(iso: string, today = new Date()): string {
  const days = Math.round((today.getTime() - new Date(iso).getTime()) / 86_400_000);
  if (days < 0) return `in ${Math.ceil(-days / 7)} wk`;
  if (days < 60) return `${days} d`;
  if (days < 730) return `${Math.round(days / 30.4)} mo`;
  return `${(days / 365.25).toFixed(1)} yr`;
}

export type KindFilter = 'pack' | 'box';
export type ShowFilter = 'all' | 'alerting' | 'floor' | 'watched';

/** Rows to show. "Needs a look" (alerting) lists every alerting product,
 *  boxes and packs alike, like the alert box; the others keep to one kind. */
export function filterRows(rows: TrendRow[], kind: KindFilter, show: ShowFilter): TrendRow[] {
  if (show === 'alerting') return rows.filter(r => r.alerts.length > 0);
  return rows.filter(r =>
    (kind === 'box') === isBox(r) &&
    (show === 'all' || (show === 'watched' ? r.watch != null : r.trend?.signal === 'floor')),
  );
}

/** Explains an alert in plain words. */
export function alertText(r: TrendRow, reason: AlertReason): string {
  if (reason === 'deal' && r.deal) {
    const lang = languageLabel(r.deal.language);
    return `${money(r.deal.price)} at ${r.deal.store}${lang ? ` (${lang})` : ''}, ${Math.round(r.deal.discount * 100)}% under market`;
  }
  const t = r.trend!;
  if (reason === 'floor') return `possible floor at ${money(t.current)}, ${pct(-t.dropFromRelease)} since release`;
  if (reason === 'dip') return `${money(t.current)}, ${Math.round(-(t.vsUsual ?? 0) * 100)}% under its usual ${money(t.usual)}`;
  if (reason === 'new_low') return `new low at ${money(t.current)}, below its previous low of ${money(t.prevLow)}`;
  return `${pct(-t.dropFromRelease)} since release, past your ${pct(-(r.watch?.dropAlert ?? 0))} alert`;
}

/** With this many packs in the collection, alerts are muted (mirrors the API). */
export const MUTE_AT_STOCK = 20;

/** Why a watched product needs attention right now (mirrors the API's rule). */
export function alertsFor(t: Trend | null, w: Watch | null, stock = 0, deal: RowDeal | null = null): AlertReason[] {
  if (!w || stock >= MUTE_AT_STOCK) return [];
  return [
    ...(t?.signal === 'floor' ? ['floor' as const] : []),
    ...(t && w.dropAlert != null && t.dropFromRelease >= w.dropAlert ? ['drop' as const] : []),
    ...(t?.newLow ? ['new_low' as const] : []),
    ...(t?.dip ? ['dip' as const] : []),
    ...(deal ? ['deal' as const] : []),
  ];
}

/** Packs in stock when that is what silences a watched product's alerts; null otherwise. */
export function mutedStock(r: TrendRow): number | null {
  const n = r.stock ?? 0;
  return n >= MUTE_AT_STOCK && alertsFor(r.trend, r.watch, 0, r.deal ?? null).length > 0 ? n : null;
}

/** Feedback right after starting to watch: would it alert now, and why? */
export function watchFeedback(r: TrendRow): { msg: string; alerting: boolean } {
  const name = `${r.setName} ${r.productType}`;
  if (r.alerts.length > 0) {
    return { msg: `Watching ${name}: alert now, ${r.alerts.map(a => alertText(r, a)).join('; ')}`, alerting: true };
  }
  if ((r.stock ?? 0) >= MUTE_AT_STOCK) {
    return { msg: `Watching ${name}. Alerts are muted: you have ${r.stock} packs.`, alerting: false };
  }
  const now = r.trend ? SIGNALS[r.trend.signal].label.toLowerCase() : 'no price history yet';
  return { msg: `Watching ${name}. No alert right now (${now}).`, alerting: false };
}

/** "$43" and "↑6%" (2-week change, when known) for a pack's contents value. */
export function evText(r: TrendRow): { value: string; change: string | null } | null {
  if (!r.ev) return null;
  const c = r.ev.change;
  const change = c == null ? null : Math.round(c * 100) === 0 ? '→ flat' : `${c > 0 ? '↑' : '↓'}${Math.abs(Math.round(c * 100))}%`;
  return { value: money(r.ev.value), change };
}

/** Below this many packs, an alert notes how many you have. */
export const LOW_STOCK = 10;

/** "you own none" / "you own 6" when you have few packs of it; null otherwise. */
export function stockNote(r: TrendRow): string | null {
  const n = r.stock ?? 0;
  if (n >= LOW_STOCK) return null;
  return n === 0 ? 'you own none' : `you own ${n}`;
}

/** Alerting rows grouped by set (sets in first-seen order); within a set,
 *  boxes before packs and regular before Collector. */
export function groupBySet(rows: TrendRow[]): { setName: string; rows: TrendRow[] }[] {
  const groups = new Map<string, TrendRow[]>();
  for (const r of rows) {
    if (!groups.has(r.setName)) groups.set(r.setName, []);
    groups.get(r.setName)!.push(r);
  }
  const rank = (r: TrendRow) => (isBox(r) ? 0 : 2) + (r.productType.includes('Collector') ? 1 : 0);
  return [...groups].map(([setName, list]) => ({
    setName,
    rows: [...list].sort((a, b) => rank(a) - rank(b) || a.productType.localeCompare(b.productType)),
  }));
}

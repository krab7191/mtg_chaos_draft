// Types and helpers for the price trends page.

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
  signal: Signal;
  spark: number[];
  sparkStart: string;
  historyDays: number;
  released: string | null;
}

export interface Watch { mtgstocksId: number; dropAlert: number | null }

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
  alerts: ('floor' | 'drop')[];
  /** Packs of this set and kind in the collection (loose packs for a box row). */
  stock?: number;
}

export interface Thresholds {
  newSetDays: number;
  trendWindow: number;
  trendMove: number;
  floorMinAge: number;
  floorDrop: number;
  floorWindow: number;
  floorBand: number;
  floorNear: number;
}

export interface TrendsResponse {
  products: TrendRow[];
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
export type ShowFilter = 'all' | 'watched' | 'floor';

export function filterRows(rows: TrendRow[], kind: KindFilter, show: ShowFilter): TrendRow[] {
  return rows.filter(r =>
    (kind === 'box') === isBox(r) &&
    (show === 'all' || (show === 'watched' ? r.watch != null : r.trend?.signal === 'floor')),
  );
}

/** Explains an alert in plain words. */
export function alertText(r: TrendRow, reason: 'floor' | 'drop'): string {
  const t = r.trend!;
  if (reason === 'floor') return `possible floor at ${money(t.current)}, ${pct(-t.dropFromRelease)} since release`;
  return `${pct(-t.dropFromRelease)} since release, past your ${pct(-(r.watch?.dropAlert ?? 0))} alert`;
}

/** Why a watched product needs attention right now (mirrors the API's rule). */
export function alertsFor(t: Trend | null, w: Watch | null): ('floor' | 'drop')[] {
  if (!t || !w) return [];
  return [
    ...(t.signal === 'floor' ? ['floor' as const] : []),
    ...(w.dropAlert != null && t.dropFromRelease >= w.dropAlert ? ['drop' as const] : []),
  ];
}

/** Feedback right after starting to watch: would it alert now, and why? */
export function watchFeedback(r: TrendRow): { msg: string; alerting: boolean } {
  const name = `${r.setName} ${r.productType}`;
  if (r.alerts.length > 0) {
    return { msg: `Watching ${name}: alert now, ${r.alerts.map(a => alertText(r, a)).join('; ')}`, alerting: true };
  }
  const now = r.trend ? SIGNALS[r.trend.signal].label.toLowerCase() : 'no price history yet';
  return { msg: `Watching ${name}. No alert right now (${now}).`, alerting: false };
}

/** Below this many packs, an alert notes how many you have. */
export const LOW_STOCK = 10;

/** "none in stock" / "6 in stock" when stock is low; null otherwise. */
export function stockNote(r: TrendRow): string | null {
  const n = r.stock ?? 0;
  if (n >= LOW_STOCK) return null;
  return n === 0 ? 'none in stock' : `${n} in stock`;
}

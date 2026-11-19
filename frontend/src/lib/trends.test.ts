import { describe, it, expect } from 'vitest';
import { perPack, money, pct, age, filterRows, mutedStock, alertText, alertsFor, watchFeedback, stockNote, describeRule, isBox, type TrendRow, type Trend } from './trends';

const trend = (over: Partial<Trend> = {}): Trend => ({
  current: 120, low: 100, lowDay: '2026-03-01', aboveLow: 0.2, change30: -0.02, releasePrice: 150,
  dropFromRelease: 0.2, band: 0.05, signal: 'stable', spark: [150, 120], sparkStart: '2026-01-01',
  historyDays: 300, released: '2026-01-01', ...over,
});
const row = (over: Partial<TrendRow> = {}): TrendRow => ({
  mtgstocksId: 1, name: 'Play Booster Display', setName: 'Set', setCode: 'S', releasedAt: '2026-01-01',
  productType: 'Play Booster Box', marketPrice: 118, packsPerBox: 36, trend: trend(), watch: null, alerts: [], ...over,
});

describe('trends helpers', () => {
  it('splits a box price per pack and passes a pack price through', () => {
    expect(perPack(row())).toBeCloseTo(120 / 36);
    expect(perPack(row({ packsPerBox: undefined, trend: null, marketPrice: 4 }))).toBe(4);
    expect(perPack(row({ trend: null, marketPrice: null }))).toBeNull();
    expect(isBox(row({ packsPerBox: 0 }))).toBe(false);
  });

  it('formats money and signed percentages', () => {
    expect(money(4.5)).toBe('$4.50');
    expect(money(null)).toBe('—');
    expect(pct(0.123)).toBe('+12%');
    expect(pct(-0.08)).toBe('−8%');
    expect(pct(0.0001)).toBe('0%');
    expect(pct(null)).toBe('—');
    expect(pct(Infinity)).toBe('—');
    expect(pct(0.1234, 1)).toBe('+12.3%');
  });

  it('describes how long ago a date was', () => {
    const today = new Date('2026-11-18T00:00:00Z');
    expect(age('2026-11-01', today)).toBe('17 d');
    expect(age('2026-05-18', today)).toBe('6 mo');
    expect(age('2024-05-18', today)).toBe('2.5 yr');
    expect(age('2026-12-09', today)).toBe('in 3 wk');
  });

  it('filters by kind and by watched or floor', () => {
    const rows = [
      row({ mtgstocksId: 1 }),
      row({ mtgstocksId: 2, packsPerBox: undefined, productType: 'Play Booster' }),
      row({ mtgstocksId: 3, watch: { mtgstocksId: 3, dropAlert: null } }),
      row({ mtgstocksId: 4, trend: trend({ signal: 'floor' }) }),
      row({ mtgstocksId: 5, trend: null }),
    ];
    const ids = (r: TrendRow[]) => r.map(x => x.mtgstocksId);
    expect(ids(filterRows(rows, 'box', 'all'))).toEqual([1, 3, 4, 5]);
    expect(ids(filterRows(rows, 'pack', 'all'))).toEqual([2]);
    expect(ids(filterRows(rows, 'box', 'watched'))).toEqual([3]);
    expect(ids(filterRows(rows, 'box', 'floor'))).toEqual([4]);
    // "Needs a look": every alerting row, boxes and packs alike.
    rows[1].alerts = ['new_low'];
    rows[3].alerts = ['floor'];
    expect(ids(filterRows(rows, 'box', 'alerting'))).toEqual([2, 4]);
    expect(ids(filterRows(rows, 'pack', 'alerting'))).toEqual([2, 4]);
  });

  it('says when plenty of stock is what mutes an alert', () => {
    const watch = { mtgstocksId: 1, dropAlert: null };
    const floor = trend({ signal: 'floor' });
    expect(mutedStock(row({ trend: floor, watch, stock: 36 }))).toBe(36);
    expect(mutedStock(row({ trend: floor, watch, stock: 19 }))).toBeNull(); // alerts normally
    expect(mutedStock(row({ trend: floor, watch: null, stock: 36 }))).toBeNull(); // not watched
    expect(mutedStock(row({ trend: trend({ signal: 'rising' }), watch, stock: 36 }))).toBeNull(); // nothing to mute
  });

  it('explains alerts in words', () => {
    const r = row({ trend: trend({ current: 90, dropFromRelease: 0.4 }), watch: { mtgstocksId: 1, dropAlert: 0.3 } });
    expect(alertText(r, 'floor')).toBe('possible floor at $90.00, −40% since release');
    expect(alertText(r, 'drop')).toBe('−40% since release, past your −30% alert');
    expect(alertText(row({ watch: null }), 'drop')).toContain('past your 0% alert');
  });

  it('works out alerts like the API does', () => {
    const w = { mtgstocksId: 1, dropAlert: 0.3 };
    expect(alertsFor(trend({ signal: 'floor', dropFromRelease: 0.35 }), w)).toEqual(['floor', 'drop']);
    expect(alertsFor(trend({ dropFromRelease: 0.1 }), w)).toEqual([]);
    expect(alertsFor(trend({ signal: 'floor' }), null)).toEqual([]);
    expect(alertsFor(null, w)).toEqual([]);
    // 20+ packs in the collection mute alerts; 19 don't.
    expect(alertsFor(trend({ signal: 'floor' }), w, 19)).toEqual(['floor']);
    expect(alertsFor(trend({ signal: 'floor' }), w, 20)).toEqual([]);
  });

  it('gives watch feedback with or without history', () => {
    expect(watchFeedback(row({ trend: null })).msg).toContain('(no price history yet)');
    expect(watchFeedback(row({ trend: trend({ signal: 'rising' }) }))).toEqual({
      msg: 'Watching Set Play Booster Box. No alert right now (rising).', alerting: false,
    });
    expect(watchFeedback(row({ stock: 36, alerts: [] }))).toEqual({
      msg: 'Watching Set Play Booster Box. Alerts are muted: you have 36 packs.', alerting: false,
    });
  });

  it('notes low stock only', () => {
    expect(stockNote(row({ stock: 0 }))).toBe('none in stock');
    expect(stockNote(row({ stock: undefined }))).toBe('none in stock');
    expect(stockNote(row({ stock: 6 }))).toBe('6 in stock');
    expect(stockNote(row({ stock: 10 }))).toBeNull();
  });

  it('describes a floor rule, leaving out a zero drop', () => {
    expect(describeRule({ minAge: 150, drop: 0.1, window: 14, band: 0.12, near: 0.1 }))
      .toBe('released at least 150 days ago, at least 10% below its release price, moving within a 12% range for the last 14 days, within 10% of its lowest price since release');
    expect(describeRule({ minAge: 150, drop: 0, window: 14, band: 0.08, near: 0.2 })).not.toContain('below its release price');
  });

  it('describes the recent-low requirement', () => {
    expect(describeRule({ minAge: 180, drop: 0.2, window: 14, band: 0.2, near: 0.05, recent: 60 }))
      .toContain('within 5% of its lowest price since release, with that low reached in the last 60 days');
  });

  it('raises and explains a new-low alert', () => {
    const t = trend({ newLow: true, prevLow: 112.5, current: 108 });
    expect(alertsFor(t, { mtgstocksId: 1, dropAlert: null })).toEqual(['new_low']);
    expect(alertText(row({ trend: t }), 'new_low')).toBe('new low at $108.00, below its previous low of $112.50');
  });
});

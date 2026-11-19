import { describe, it, expect } from 'vitest';
import { rank, topPicks, reasons, type Candidate } from './bestdeals';
import { NO_FILTERS } from './deals';

const cand = (over: Partial<Candidate> = {}): Candidate => ({
  mtgstocksId: 1, setName: 'Set A', productType: 'Booster Pack', releasedAt: '2007-01-01', store: 'CardTrader',
  url: 'u', title: 't', language: 'en', price: 10, market: 12, contents: 12, signal: 'stable', timing: 1, ...over,
});
const ship = { perOrder: { CardTrader: 6, 'Card Kingdom': 0, TCGplayer: 0 }, packsPerOrder: 3 };

describe('best deals ranking', () => {
  it('scores contents per delivered dollar, times the timing factor', () => {
    const [r] = rank([cand()], NO_FILTERS, ship);
    expect(r.delivered).toBe(12); // $10 + $6 / 3 packs
    expect(r.perDollar).toBe(1);
    expect(rank([cand({ timing: 1.1 })], NO_FILTERS, ship)[0].score).toBeCloseTo(1.1);
  });

  it('shipping can change the order', () => {
    const ct = cand({ mtgstocksId: 1, setName: 'A', store: 'CardTrader', price: 9, contents: 12 });   // 12 / 11 = 1.09
    const ck = cand({ mtgstocksId: 2, setName: 'B', store: 'Card Kingdom', price: 10, contents: 12 }); // 12 / 10 = 1.2
    expect(rank([ct, ck], NO_FILTERS, ship).map(r => r.setName)).toEqual(['B', 'A']);
    expect(rank([ct, ck], NO_FILTERS, { ...ship, perOrder: { CardTrader: 0 } }).map(r => r.setName)).toEqual(['A', 'B']);
  });

  it('keeps 3 different sets, and an excluded language is replaced by the next pack', () => {
    const all = [
      cand({ mtgstocksId: 1, setName: 'A', contents: 30, language: 'fr' }),
      cand({ mtgstocksId: 1, setName: 'A', contents: 28, url: 'a2' }),
      cand({ mtgstocksId: 2, setName: 'B', contents: 25 }),
      cand({ mtgstocksId: 3, setName: 'C', contents: 20, language: 'fr' }),
      cand({ mtgstocksId: 4, setName: 'D', contents: 15 }),
    ];
    expect(topPicks(rank(all, NO_FILTERS, ship)).map(r => `${r.setName}/${r.language}`)).toEqual(['A/fr', 'B/en', 'C/fr']);
    const noFrench = topPicks(rank(all, { ...NO_FILTERS, languages: ['en'] }, ship));
    expect(noFrench.map(r => `${r.setName}/${r.language}`)).toEqual(['A/en', 'B/en', 'D/en']);
    expect(topPicks(rank(all, { ...NO_FILTERS, maxPrice: 5 }, ship))).toEqual([]);
  });

  it('explains each pick', () => {
    const [r] = rank([cand({ price: 9, market: 12, contents: 13.66, signal: 'climbing', timing: 1.082, evChange: 0.05 })], NO_FILTERS, ship);
    const why = reasons(r);
    expect(why[0]).toBe('$1.24 of cards per $1: contents $13.66 for $11.00 delivered');
    expect(why).toContain("25% under TCGplayer's market price ($12.00)");
    expect(why.some(w => w.startsWith('↗ Climbing: packs like this were up ~8%'))).toBe(true);
    expect(why).toContain('Contents ↑5% over 2 weeks');
    const tcg = reasons(rank([cand({ store: 'TCGplayer', price: 12, signal: 'new', timing: 0.935 })], NO_FILTERS, ship)[0]);
    expect(tcg.some(w => w.includes('under'))).toBe(false);
    expect(tcg.some(w => w.startsWith('New set: usually ~7% cheaper'))).toBe(true); // 0.935: 6.5% rounds up
    expect(reasons(rank([cand({ signal: 'floor' })], NO_FILTERS, ship)[0])).toContain('◆ Possible floor: near its low');
    expect(reasons(rank([cand({ signal: 'falling', timing: 1.045 })], NO_FILTERS, ship)[0]).some(w => w.includes('+5%'))).toBe(true);
    expect(reasons(rank([cand({ signal: '', market: 10 })], NO_FILTERS, ship)[0])).toHaveLength(1); // at market, no signal: just the value
  });
});

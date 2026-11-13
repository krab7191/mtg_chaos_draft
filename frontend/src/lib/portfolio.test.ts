import { describe, it, expect } from 'vitest';
import { portfolio, activity, type Disposal, type StockPack } from './portfolio';
import type { Purchase } from './purchases';

const buy = (over: Partial<Purchase> = {}): Purchase => ({
  id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 10,
  totalPrice: 40, fees: 0, unitPrice: 4, currentPrice: 5, purchasedOn: '2026-10-01', ...over,
});
const out = (over: Partial<Disposal> = {}): Disposal => ({
  id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', kind: 'draft', draftId: 7,
  quantity: 2, proceeds: 10, marketValue: 10, fees: 0, disposedOn: '2026-11-01', ...over,
});

const stock = (quantity: number, over: Partial<StockPack> = {}): StockPack => ({
  id: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity, marketPrice: 5, ...over,
});

describe('portfolio', () => {
  it('values what is in stock at average cost, unrealized against today', () => {
    const pf = portfolio([buy(), buy({ id: 2, quantity: 10, totalPrice: 60, purchasedOn: '2026-10-05' })], [], [stock(20)]);
    const [pos] = pf.positions;
    expect(pos).toMatchObject({ bought: 20, stock: 20, costed: 20, opened: 0, cost: 100, avgCost: 5, value: 100, unrealized: 0 });
    expect(pf.stockPacks).toBe(20);
    expect(pf.invested).toBe(100);
    expect(pf.realized).toBe(0);
  });

  it('counts logged packs no longer in stock as opened, not a loss', () => {
    // Crimson Vow: 30 bought for $237.26, 8 left, nothing recorded as sold.
    const pf = portfolio(
      [buy({ quantity: 30, totalPrice: 237.26 })],
      [],
      [stock(8, { marketPrice: 9.79 })],
    );
    const [pos] = pf.positions;
    expect(pos).toMatchObject({ stock: 8, costed: 8, opened: 22, uncosted: 0 });
    expect(pos.openedCost).toBeCloseTo(173.99, 2);
    expect(pos.cost).toBeCloseTo(63.27, 2);
    expect(pos.value).toBeCloseTo(78.32, 2);
    expect(pos.unrealized).toBeCloseTo(15.05, 2);
    expect(pf.realized).toBe(0);
    expect(pf.openedPacks).toBe(22);
  });

  it('realizes gains on drafts and sales at the average cost at the time', () => {
    const pf = portfolio(
      [buy({ quantity: 10, totalPrice: 40 })],
      [
        out({ id: 1, quantity: 2, proceeds: 10 }),                                      // cost 8 → +2
        out({ id: 2, kind: 'sale', draftId: null, quantity: 3, proceeds: 18, fees: 3 }), // cost 12, net 15 → +3
      ],
      [stock(5)],
    );
    expect(pf.results.get(1)).toMatchObject({ basisPacks: 2, unknownPacks: 0, cost: 8, gain: 2 });
    expect(pf.results.get(2)?.gain).toBeCloseTo(3);
    expect(pf.realized).toBeCloseTo(5);
    expect(pf.proceeds).toBe(25);
    expect(pf.positions[0]).toMatchObject({ stock: 5, disposed: 5, opened: 0, cost: 20, value: 25 });
    expect(pf.unrealized).toBe(5);
    expect(pf.unrealizedPct).toBeCloseTo(0.25);
  });

  it('shows the loss on a draft where players paid less than market', () => {
    // 6 players × 3 packs; $160 at market, $60 collected.
    const pf = portfolio(
      [buy({ quantity: 18, totalPrice: 130 })],
      [out({ quantity: 18, proceeds: 60, marketValue: 160 })],
      [],
    );
    expect(pf.realized).toBeCloseTo(-70);
    expect(pf.positions[0]).toMatchObject({ stock: 0, value: null, opened: 0 });
  });

  it('values stock beyond the logged packs but leaves it out of cost and gains', () => {
    const pf = portfolio([buy({ quantity: 2, totalPrice: 8 })], [], [stock(5)]);
    expect(pf.positions[0]).toMatchObject({ stock: 5, costed: 2, uncosted: 3, cost: 8, value: 25, unrealized: 2 });
    expect(pf.uncostedPacks).toBe(3);
    expect(pf.value).toBe(25);
  });

  it('lists packs in stock with no purchases at all', () => {
    const pf = portfolio([], [], [stock(4, { id: 9, setName: 'Unlogged' }), stock(0, { id: 8 })]);
    expect(pf.positions).toHaveLength(1);
    expect(pf.positions[0]).toMatchObject({ setName: 'Unlogged', stock: 4, costed: 0, value: 20, unrealized: null });
    expect(pf.unrealizedPct).toBeNull();
  });

  it('leaves packs with no logged purchase out of gains', () => {
    const pf = portfolio(
      [buy({ quantity: 2, totalPrice: 8 })],
      [out({ quantity: 4, proceeds: 20 })], // 2 with a cost, 2 without
      [],
    );
    expect(pf.results.get(1)).toMatchObject({ basisPacks: 2, unknownPacks: 2, cost: 8 });
    expect(pf.results.get(1)?.gain).toBeCloseTo(2); // half the proceeds (10) − 8
    expect(pf.unknownPacks).toBe(2);
  });

  it('gives no gain when nothing was logged before the packs left', () => {
    const pf = portfolio(
      [buy({ purchasedOn: '2026-12-01' })],
      [out({ disposedOn: '2026-11-01' })],
      [stock(10)],
    );
    expect(pf.results.get(1)?.gain).toBeNull();
    expect(pf.realized).toBe(0);
    expect(pf.positions[0].costed).toBe(10);
  });

  it('counts a same-day purchase before packs leave', () => {
    const pf = portfolio([buy({ purchasedOn: '2026-11-01' })], [out({ disposedOn: '2026-11-01' })], [stock(8)]);
    expect(pf.results.get(1)?.gain).toBeCloseTo(2);
  });

  it('leaves tax & shipping out of cost but reports it', () => {
    const pf = portfolio([buy({ quantity: 30, totalPrice: 132.18, fees: 10.68 })], [], [stock(30)]);
    expect(pf.positions[0].cost).toBeCloseTo(121.5);
    expect(pf.purchaseFees).toBeCloseTo(10.68);
    expect(pf.invested).toBeCloseTo(132.18);
  });

  it('falls back to the purchase price, and leaves unpriced stock out of value', () => {
    const fromPurchase = portfolio([buy({ currentPrice: 6 })], [], [stock(10, { marketPrice: null })]);
    expect(fromPurchase.positions[0].value).toBe(60);
    const unpriced = portfolio([buy({ currentPrice: null })], [], [stock(10, { marketPrice: null })]);
    expect(unpriced.value).toBe(0);
    expect(unpriced.pricedCost).toBe(0);
    expect(unpriced.unrealizedPct).toBeNull();
    expect(unpriced.cost).toBe(40);
  });

  it('lists the biggest holdings first, then sold-out packs', () => {
    const pf = portfolio(
      [
        buy({ id: 1, packId: 1, quantity: 1, totalPrice: 4 }),
        buy({ id: 2, packId: 2, quantity: 10, totalPrice: 50 }),
        buy({ id: 3, packId: 3, quantity: 1, totalPrice: 4 }),
      ],
      [out({ packId: 3, quantity: 1, proceeds: 5 })],
      [stock(1, { id: 1, setName: 'Small' }), stock(10, { id: 2, setName: 'Big' })],
    );
    expect(pf.positions.map(p => p.setName)).toEqual(['Big', 'Small', 'Bloomburrow']);
  });

  it('names a position after its first event when it is not in stock', () => {
    const pf = portfolio([], [out({ setName: 'Only Out' })], []);
    expect(pf.positions[0]).toMatchObject({ setName: 'Only Out', stock: 0, bought: 0, disposed: 2 });
  });
});

describe('activity', () => {
  it('merges buys, sales, and one line per draft, newest first', () => {
    const rows = activity(
      [buy({ id: 1, purchasedOn: '2026-11-01' })],
      [
        out({ id: 10, packId: 1, draftId: 7, disposedOn: '2026-11-01' }),
        out({ id: 11, packId: 2, draftId: 7, disposedOn: '2026-11-01' }),
        out({ id: 12, kind: 'sale', draftId: null, disposedOn: '2026-11-05' }),
        out({ id: 13, draftId: null, disposedOn: '2026-10-01' }), // from a deleted draft
      ],
    );
    expect(rows.map(r => r.key)).toEqual(['s12', 'd7', 'b1', 'x13']);
    const draft = rows[1];
    expect(draft.type === 'draft' && draft.disposals.length).toBe(2);
    expect(draft.type === 'draft' && draft.draftId).toBe(7);
  });
});

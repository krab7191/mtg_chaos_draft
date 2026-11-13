import { describe, it, expect } from 'vitest';
import { portfolio, activity, type Disposal } from './portfolio';
import type { Purchase } from './purchases';

const buy = (over: Partial<Purchase> = {}): Purchase => ({
  id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 10,
  totalPrice: 40, fees: 0, unitPrice: 4, currentPrice: 5, purchasedOn: '2026-10-01', ...over,
});
const out = (over: Partial<Disposal> = {}): Disposal => ({
  id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', kind: 'draft', draftId: 7,
  quantity: 2, proceeds: 10, marketValue: 10, fees: 0, disposedOn: '2026-11-01', ...over,
});

describe('portfolio', () => {
  it('values packs still held at average cost, unrealized against today', () => {
    const pf = portfolio([buy(), buy({ id: 2, quantity: 10, totalPrice: 60, purchasedOn: '2026-10-05' })], []);
    const [pos] = pf.positions;
    expect(pos).toMatchObject({ bought: 20, held: 20, cost: 100, avgCost: 5, value: 100, unrealized: 0 });
    expect(pf.heldPacks).toBe(20);
    expect(pf.invested).toBe(100);
    expect(pf.realized).toBe(0);
  });

  it('realizes gains on drafts and sales at the average cost at the time', () => {
    const pf = portfolio(
      [buy({ quantity: 10, totalPrice: 40 })],
      [
        out({ id: 1, quantity: 2, proceeds: 10 }),                                      // cost 8 → +2
        out({ id: 2, kind: 'sale', draftId: null, quantity: 3, proceeds: 18, fees: 3 }), // cost 12, net 15 → +3
      ],
    );
    expect(pf.results.get(1)).toMatchObject({ basisPacks: 2, unknownPacks: 0, cost: 8, gain: 2 });
    expect(pf.results.get(2)?.gain).toBeCloseTo(3);
    expect(pf.realized).toBeCloseTo(5);
    expect(pf.proceeds).toBe(25);
    const [pos] = pf.positions;
    expect(pos).toMatchObject({ held: 5, disposed: 5, cost: 20, value: 25 });
    expect(pf.unrealized).toBe(5);
    expect(pf.unrealizedPct).toBeCloseTo(0.25);
  });

  it('shows the loss on a draft where players paid less than market', () => {
    // 6 players × 3 packs; $160 at market, $60 collected.
    const pf = portfolio(
      [buy({ quantity: 18, totalPrice: 130, currentPrice: 8.9 })],
      [out({ quantity: 18, proceeds: 60, marketValue: 160 })],
    );
    expect(pf.realized).toBeCloseTo(-70);
    expect(pf.positions[0].held).toBe(0);
    expect(pf.positions[0].value).toBeNull();
    expect(pf.value).toBe(0);
  });

  it('leaves packs with no logged purchase out of gains', () => {
    const pf = portfolio(
      [buy({ quantity: 2, totalPrice: 8 })],
      [out({ quantity: 4, proceeds: 20 })], // 2 with a cost, 2 without
    );
    expect(pf.results.get(1)).toMatchObject({ basisPacks: 2, unknownPacks: 2, cost: 8 });
    expect(pf.results.get(1)?.gain).toBeCloseTo(2); // half the proceeds (10) − 8
    expect(pf.unknownPacks).toBe(2);
  });

  it('gives no gain when nothing was logged before the packs left', () => {
    const pf = portfolio(
      [buy({ purchasedOn: '2026-12-01' })],
      [out({ disposedOn: '2026-11-01' })],
    );
    expect(pf.results.get(1)?.gain).toBeNull();
    expect(pf.realized).toBe(0);
    expect(pf.positions[0].held).toBe(10);
  });

  it('counts a same-day purchase before packs leave', () => {
    const pf = portfolio([buy({ purchasedOn: '2026-11-01' })], [out({ disposedOn: '2026-11-01' })]);
    expect(pf.results.get(1)?.gain).toBeCloseTo(2);
  });

  it('leaves tax & shipping out of cost but reports it', () => {
    const pf = portfolio([buy({ quantity: 30, totalPrice: 132.18, fees: 10.68, currentPrice: 5 })], []);
    expect(pf.positions[0].cost).toBeCloseTo(121.5);
    expect(pf.purchaseFees).toBeCloseTo(10.68);
    expect(pf.invested).toBeCloseTo(132.18);
  });

  it('keeps unpriced holdings out of value', () => {
    const pf = portfolio([buy({ currentPrice: null })], []);
    expect(pf.value).toBe(0);
    expect(pf.pricedCost).toBe(0);
    expect(pf.unrealizedPct).toBeNull();
    expect(pf.cost).toBe(40);
  });

  it('lists open positions by cost, then closed ones', () => {
    const pf = portfolio(
      [
        buy({ id: 1, packId: 1, setName: 'Small', quantity: 1, totalPrice: 4 }),
        buy({ id: 2, packId: 2, setName: 'Big', quantity: 10, totalPrice: 50 }),
        buy({ id: 3, packId: 3, setName: 'Gone', quantity: 1, totalPrice: 4 }),
      ],
      [out({ packId: 3, setName: 'Gone', quantity: 1, proceeds: 5 })],
    );
    expect(pf.positions.map(p => p.setName)).toEqual(['Big', 'Small', 'Gone']);
  });

  it('names a position after its first event, even a disposal', () => {
    const pf = portfolio([], [out({ setName: 'Only Out' })]);
    expect(pf.positions[0]).toMatchObject({ setName: 'Only Out', held: 0, bought: 0, disposed: 2 });
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

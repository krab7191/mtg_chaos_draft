import { describe, it, expect } from 'vitest';
import { groupPicks, type PickLike } from './draft';

const pick = (packId: number | null, setName: string, extra: Partial<PickLike> = {}): PickLike => ({
  packId, setName, productType: 'Draft Booster', marketPrice: 5, ...extra,
});

describe('groupPicks', () => {
  it('combines repeated packs into one line in first-picked order', () => {
    const groups = groupPicks([
      pick(1, 'Unfinity', { marketPrice: 6.65, unitPrice: 6.65 }),
      pick(2, 'Hobbit'),
      pick(1, 'Unfinity', { marketPrice: 6.65, unitPrice: 6.65 }),
      pick(1, 'Unfinity', { marketPrice: 6.65, unitPrice: 6.65 }),
    ]);
    expect(groups.map(g => g.setName)).toEqual(['Unfinity', 'Hobbit']);
    const u = groups[0];
    expect([u.slots, u.packs, u.qty, u.each]).toEqual([3, 3, 3, 6.65]);
    expect(u.total).toBeCloseTo(19.95);
  });

  it('counts physical packs for small packs (two Aftermath slots = 6 packs)', () => {
    const aftermath = { marketPrice: 19.32, unitPrice: 6.44, packCount: 3 };
    const [g] = groupPicks([pick(7, 'Aftermath', aftermath), pick(7, 'Aftermath', aftermath)]);
    expect([g.slots, g.packs, g.qty, g.each]).toEqual([2, 6, 6, 6.44]);
    expect(g.total).toBeCloseTo(38.64);
  });

  it('falls back to slot price when the unit price is unknown (saved drafts)', () => {
    const [g] = groupPicks([pick(3, 'A', { marketPrice: 4 }), pick(3, 'A', { marketPrice: 4 })]);
    expect([g.qty, g.each, g.total]).toEqual([2, 4, 8]);
  });

  it('shows only the total when prices changed during the draft', () => {
    const [g] = groupPicks([pick(3, 'A', { marketPrice: 4, unitPrice: 4 }), pick(3, 'A', { marketPrice: 5, unitPrice: 5 })]);
    expect([g.each, g.total]).toEqual([null, 9]);
  });

  it('tracks unpriced and still-pricing picks', () => {
    const [g] = groupPicks([pick(3, 'A', { marketPrice: null }), pick(3, 'A', { marketPrice: 4, pricing: true })]);
    expect([g.unpriced, g.total, g.pricing]).toEqual([1, 4, true]);
    const [none] = groupPicks([pick(4, 'B', { marketPrice: null })]);
    expect(none.total).toBeNull();
  });

  it('groups picks without a pack id by name and type', () => {
    const groups = groupPicks([
      pick(null, 'Old', { productType: 'Booster' }),
      pick(null, 'Old', { productType: 'Booster' }),
      pick(null, 'Old', { productType: 'Set Booster' }),
    ]);
    expect(groups.map(g => g.slots)).toEqual([2, 1]);
  });
});

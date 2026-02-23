import { describe, it, expect } from 'vitest';
import { computeSortedSets, type Pack } from './collection';

const p = (id: number, setName: string, qty: number, price: number | null = 10): Pack => ({
  id, setName, productType: 'draft_booster', quantity: qty, marketPrice: price,
});

describe('computeSortedSets', () => {
  describe('name sort', () => {
    it('groups packs by setName', () => {
      const packs = [p(1, 'Zendikar', 2), p(2, 'Zendikar', 1), p(3, 'Alpha', 3)];
      const result = computeSortedSets(packs, 'name', 'asc');
      expect(result.map(([k]) => k)).toEqual(['Alpha', 'Zendikar']);
    });

    it('sorts sets descending', () => {
      const packs = [p(1, 'Alpha', 1), p(2, 'Zendikar', 1)];
      const result = computeSortedSets(packs, 'name', 'desc');
      expect(result[0][0]).toBe('Zendikar');
    });

    it('puts zero-qty packs last within a set', () => {
      const packs = [p(1, 'Alpha', 0), p(2, 'Alpha', 2)];
      const [, setPacks] = computeSortedSets(packs, 'name', 'asc')[0];
      expect(setPacks[0].id).toBe(2);
    });
  });

  describe('qty sort', () => {
    it('returns each pack as its own group', () => {
      const packs = [p(1, 'Alpha', 3), p(2, 'Beta', 1)];
      const result = computeSortedSets(packs, 'qty', 'asc');
      expect(result).toHaveLength(2);
      expect(result.every(([, ps]) => ps.length === 1)).toBe(true);
    });

    it('sorts ascending by quantity', () => {
      const packs = [p(1, 'Alpha', 3), p(2, 'Beta', 1)];
      const result = computeSortedSets(packs, 'qty', 'asc');
      expect(result[0][1][0].id).toBe(2);
    });

    it('sorts descending by quantity', () => {
      const packs = [p(1, 'Alpha', 1), p(2, 'Beta', 3)];
      const result = computeSortedSets(packs, 'qty', 'desc');
      expect(result[0][1][0].id).toBe(2);
    });
  });

  describe('price sort', () => {
    it('sorts ascending by price', () => {
      const packs = [p(1, 'Alpha', 1, 20), p(2, 'Beta', 1, 5)];
      const result = computeSortedSets(packs, 'price', 'asc');
      expect(result[0][1][0].id).toBe(2);
    });

    it('treats null price as 0', () => {
      const packs = [p(1, 'Alpha', 1, null), p(2, 'Beta', 1, 5)];
      const result = computeSortedSets(packs, 'price', 'asc');
      expect(result[0][1][0].id).toBe(1);
    });
  });
});

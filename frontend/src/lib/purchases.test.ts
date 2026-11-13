import { describe, it, expect } from 'vitest';
import {
  packCost, discount, verdict, formatDiscount, returnPct, formatReturn,
  priceFields, byNewest, localToday, type Purchase,
} from './purchases';

const purchase = (over: Partial<Purchase> = {}): Purchase => ({
  id: 1, packId: 1, setName: 'S', productType: 'Play Booster', quantity: 1,
  totalPrice: 4, fees: 0, unitPrice: 4, currentPrice: 6, purchasedOn: '2026-11-01', ...over,
});

describe('packCost', () => {
  it('leaves out tax & shipping', () => {
    expect(packCost({ totalPrice: 132.18, fees: 10.68 })).toBeCloseTo(121.5);
    expect(packCost({ totalPrice: 10, fees: 0 })).toBe(10);
  });
});

describe('discount / verdict / formatDiscount', () => {
  it('is the fraction under market, negative when over', () => {
    expect(discount(4, 5)).toBeCloseTo(0.2);
    expect(discount(5.5, 5)).toBeCloseTo(-0.1);
    expect(discount(4, null)).toBeNull();
    expect(discount(4, 0)).toBeNull();
  });
  it('splits good / fair / bad at ±5%', () => {
    expect(verdict(0.05)).toBe('good');
    expect(verdict(0.04)).toBe('fair');
    expect(verdict(-0.05)).toBe('bad');
    expect(verdict(null)).toBeNull();
  });
  it('formats under, over, at market, and unknown', () => {
    expect(formatDiscount(0.2)).toBe('20% under');
    expect(formatDiscount(-0.1)).toBe('10% over');
    expect(formatDiscount(0.001)).toBe('at market');
    expect(formatDiscount(null)).toBe('—');
  });
});

describe('returnPct / formatReturn', () => {
  it('is gain over cost', () => {
    expect(returnPct(125, 100)).toBeCloseTo(0.25);
    expect(returnPct(10, 0)).toBeNull();
  });
  it('formats with a sign and one decimal', () => {
    expect(formatReturn(0.25)).toBe('+25%');
    expect(formatReturn(-0.123)).toBe('−12.3%');
    expect(formatReturn(0)).toBe('0%');
    expect(formatReturn(null)).toBe('—');
  });
});

describe('helpers', () => {
  it('priceFields picks the API field for the mode', () => {
    expect(priceFields('total', 132.18)).toEqual({ paidTotal: 132.18 });
    expect(priceFields('each', 4.41)).toEqual({ paidPrice: 4.41 });
  });
  it('byNewest sorts by date then id, newest first', () => {
    const list = [
      purchase({ id: 1, purchasedOn: '2026-01-01' }),
      purchase({ id: 2, purchasedOn: '2026-02-01' }),
      purchase({ id: 3, purchasedOn: '2026-02-01' }),
    ].sort(byNewest);
    expect(list.map(p => p.id)).toEqual([3, 2, 1]);
  });
  it('localToday formats the local date', () => {
    expect(localToday(new Date(2026, 0, 5, 23, 30))).toBe('2026-01-05');
  });
});

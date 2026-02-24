import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '../../tests/svelte';
import WeightsList from './WeightsList.svelte';

const packs = [
  { id: 1, setName: 'Alpha', productType: 'Draft Booster', marketPrice: 10.00, quantity: 2 },
  { id: 2, setName: 'Beta', productType: 'Set Booster', marketPrice: 5.00, quantity: 1 },
];

const settings = { priceFloor: 0, priceCap: 0, quantityCap: 0, packWeights: {} };

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, statusText: '' }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('WeightsList', () => {
  it('renders price-floor input', () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(container.querySelector('#price-floor')).not.toBeNull();
  });

  it('renders price-cap input', () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(container.querySelector('#price-cap')).not.toBeNull();
  });

  it('renders qty-cap input', () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(container.querySelector('#qty-cap')).not.toBeNull();
  });

  it('renders sort buttons', () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    const sortBtns = container.querySelectorAll('.sort-btn');
    expect(sortBtns.length).toBeGreaterThanOrEqual(3);
  });

  it('renders SetCard for each set', () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    // Alpha and Beta are separate sets → 2 SetCard components
    expect(container.querySelectorAll('.set-group')).toHaveLength(2);
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import WeightsList from './WeightsList.svelte';

const packs = [
  { id: 1, setName: 'Alpha', productType: 'Draft Booster', marketPrice: 10.00, quantity: 2, cardsPerPack: 15 },
  { id: 2, setName: 'Beta', productType: 'Set Booster', marketPrice: 5.00, quantity: 1, cardsPerPack: 15 },
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

  it('clicking Price sort activates price sort', async () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    const buttons = container.querySelectorAll('.sort-btn');
    await act(() => fireEvent.click(buttons[1])); // Price is second button
    expect(buttons[1].classList.contains('sort-btn--active')).toBe(true);
  });

  it('clicking Odds sort activates weight sort', async () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    const buttons = container.querySelectorAll('.sort-btn');
    await act(() => fireEvent.click(buttons[2])); // Odds is third button
    expect(buttons[2].classList.contains('sort-btn--active')).toBe(true);
  });

  it('clicking active Set sort toggles direction to desc', async () => {
    const { container } = render(WeightsList, { props: { packs, settings } });
    const buttons = container.querySelectorAll('.sort-btn');
    await act(() => fireEvent.click(buttons[0])); // Set is already active, toggle to desc
    expect(buttons[0].textContent).toContain('↓');
  });

  it('clicking Save calls fetch', async () => {
    const mockFetch = vi.fn().mockResolvedValue({ ok: true, statusText: '' });
    vi.stubGlobal('fetch', mockFetch);
    const { container } = render(WeightsList, { props: { packs, settings } });
    const saveBtn = container.querySelector('.btn-save') as HTMLButtonElement;
    await act(() => fireEvent.click(saveBtn));
    expect(mockFetch).toHaveBeenCalled();
  });

  it('clicking Save on fetch failure does not throw', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, statusText: 'Error' }));
    const { container } = render(WeightsList, { props: { packs, settings } });
    const saveBtn = container.querySelector('.btn-save') as HTMLButtonElement;
    await act(() => fireEvent.click(saveBtn));
    expect(saveBtn).not.toBeNull(); // still rendered
  });

  it('renders with non-zero settings values', () => {
    const nonZeroSettings = { priceFloor: 3, priceCap: 20, quantityCap: 5, packWeights: { '1': 2 } };
    const { container } = render(WeightsList, { props: { packs, settings: nonZeroSettings } });
    const floorInput = container.querySelector('#price-floor') as HTMLInputElement;
    expect(floorInput.value).toBe('3');
  });
});

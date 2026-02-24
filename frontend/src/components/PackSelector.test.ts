import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '../tests/svelte';
import PackSelector from './PackSelector.svelte';

const settings = { priceFloor: 0, priceCap: 0, quantityCap: 0, packWeights: {} };

const packs = [
  { id: 1, name: 'Zendikar Draft Booster', setName: 'Zendikar', productType: 'Draft Booster', quantity: 5, marketPrice: 4.99 },
  { id: 2, name: 'Alpha Set Booster', setName: 'Alpha', productType: 'Set Booster', quantity: 3, marketPrice: 8.00 },
];

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('PackSelector', () => {
  it('shows empty message when no packs', () => {
    const { container } = render(PackSelector, { props: { packs: [], settings } });
    expect(container.querySelector('.empty')).not.toBeNull();
  });

  it('renders pack list when packs provided', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    expect(container.querySelector('.pack-list')).not.toBeNull();
  });

  it('renders count pill', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    expect(container.querySelector('.count-pill')).not.toBeNull();
  });

  it('renders sort buttons', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const sortBtns = container.querySelectorAll('.sort-btn');
    expect(sortBtns.length).toBeGreaterThanOrEqual(2);
  });

  it('renders Pick a Pack button', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Pick a Pack'),
    );
    expect(btn).not.toBeNull();
  });

  it('Pick a Pack button is enabled when packs are checked', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Pick a Pack'),
    ) as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
  });

  it('renders select all and deselect all buttons', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const buttons = Array.from(container.querySelectorAll('button')).map((b) => b.textContent);
    expect(buttons.some((t) => t?.includes('Select all'))).toBe(true);
    expect(buttons.some((t) => t?.includes('Deselect all'))).toBe(true);
  });
});

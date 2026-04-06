import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '../../tests/svelte';
import CollectionList from './CollectionList.svelte';

const packs = [
  { id: 1, setName: 'Alpha', productType: 'Draft Booster', marketPrice: 10.00, quantity: 2, cardsPerPack: 15 },
  { id: 2, setName: 'Beta', productType: 'Draft Booster', marketPrice: 5.00, quantity: 1, cardsPerPack: 15 },
];

describe('CollectionList', () => {
  it('shows empty message when no packs', () => {
    const { container } = render(CollectionList, { props: { packs: [] } });
    expect(container.querySelector('.collection__empty')).not.toBeNull();
  });

  it('renders sort buttons when packs present', () => {
    const { container } = render(CollectionList, { props: { packs } });
    const sortBtns = container.querySelectorAll('.sort-btn');
    expect(sortBtns.length).toBeGreaterThanOrEqual(3);
  });

  it('renders pack count in header', () => {
    const { container } = render(CollectionList, { props: { packs } });
    // 2 + 1 = 3 total
    expect(container.querySelector('.collection__value')?.textContent).toContain('3');
  });

  it('renders CollectionCard components for each set', () => {
    const { container } = render(CollectionList, { props: { packs } });
    expect(container.querySelectorAll('.set-group')).toHaveLength(2);
  });

  it('shows footnote when any pack has non-standard cardsPerPack', () => {
    const nonStandardPacks = [{ ...packs[0], cardsPerPack: 5 }, packs[1]];
    const { container } = render(CollectionList, { props: { packs: nonStandardPacks } });
    expect(container.querySelector('.collection__footnote')).not.toBeNull();
  });

  it('does not show footnote when all packs are standard', () => {
    const { container } = render(CollectionList, { props: { packs } });
    expect(container.querySelector('.collection__footnote')).toBeNull();
  });

  it('clicking Sort by Price activates price sort', async () => {
    const { container } = render(CollectionList, { props: { packs } });
    const buttons = Array.from(container.querySelectorAll<HTMLButtonElement>('.sort-btn'));
    const priceBtn = buttons.find(b => b.textContent?.trim() === 'Price');
    expect(priceBtn).toBeDefined();
    await fireEvent.click(priceBtn!);
    expect(priceBtn!.classList.contains('sort-btn--active')).toBe(true);
  });

  it('clicking active sort button toggles direction to descending', async () => {
    const { container } = render(CollectionList, { props: { packs } });
    // 'Set' / 'name' is active by default; clicking it twice toggles asc → desc
    const buttons = Array.from(container.querySelectorAll<HTMLButtonElement>('.sort-btn'));
    const setBtn = buttons.find(b => b.textContent?.includes('Set'));
    expect(setBtn).toBeDefined();
    // First click: already active (name key), so direction flips to desc
    await fireEvent.click(setBtn!);
    // After first click it goes to desc — label becomes 'Set ↓'
    expect(setBtn!.textContent).toContain('↓');
  });

  it('shows total value when packs have price', () => {
    const { container } = render(CollectionList, { props: { packs } });
    // totalValue = 10*2 + 5*1 = $25.00; multiple .collection__value spans exist
    const valueEls = container.querySelectorAll('.collection__value');
    const texts = Array.from(valueEls).map(el => el.textContent ?? '');
    expect(texts.some(t => t.includes('$'))).toBe(true);
  });

  it('clicking Qty sort activates qty sort', async () => {
    const { container } = render(CollectionList, { props: { packs } });
    const buttons = Array.from(container.querySelectorAll<HTMLButtonElement>('.sort-btn'));
    const qtyBtn = buttons.find(b => b.textContent?.trim() === 'Qty');
    expect(qtyBtn).toBeDefined();
    await fireEvent.click(qtyBtn!);
    expect(qtyBtn!.classList.contains('sort-btn--active')).toBe(true);
  });
});

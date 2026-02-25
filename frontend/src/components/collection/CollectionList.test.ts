import { describe, it, expect } from 'vitest';
import { render } from '../../tests/svelte';
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
});

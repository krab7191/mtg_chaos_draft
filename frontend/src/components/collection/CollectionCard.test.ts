import { describe, it, expect, vi } from 'vitest';
import { render } from '../../tests/svelte';
import CollectionCard from './CollectionCard.svelte';

const packs = [
  { id: 1, productType: 'Draft Booster', marketPrice: 4.99, quantity: 2, setName: 'Zendikar', cardsPerPack: 15 },
  { id: 2, productType: 'Set Booster', marketPrice: 7.50, quantity: 1, setName: 'Zendikar', cardsPerPack: 15 },
];

describe('CollectionCard', () => {
  it('renders set name', () => {
    const { container } = render(CollectionCard, {
      props: { setName: 'Zendikar', packs, onQtyChange: vi.fn(), onDelete: vi.fn() },
    });
    expect(container.querySelector('.set-group__name')?.textContent).toBe('Zendikar');
  });

  it('renders one row per pack', () => {
    const { container } = render(CollectionCard, {
      props: { setName: 'Zendikar', packs, onQtyChange: vi.fn(), onDelete: vi.fn() },
    });
    expect(container.querySelectorAll('.pack-row')).toHaveLength(2);
  });

  it('shows set code when provided', () => {
    const packsWithCode = packs.map(p => ({ ...p, setCode: 'ZNR' }));
    const { container } = render(CollectionCard, {
      props: { setName: 'Zendikar', packs: packsWithCode, onQtyChange: vi.fn(), onDelete: vi.fn() },
    });
    const codeEl = container.querySelector('.set-group__code');
    expect(codeEl).not.toBeNull();
    expect(codeEl?.textContent).toContain('ZNR');
  });

  it('hides set code when not provided', () => {
    const { container } = render(CollectionCard, {
      props: { setName: 'Zendikar', packs, onQtyChange: vi.fn(), onDelete: vi.fn() },
    });
    expect(container.querySelector('.set-group__code')).toBeNull();
  });
});

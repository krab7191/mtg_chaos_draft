import { describe, it, expect, vi } from 'vitest';
import { render } from '../../tests/svelte';
import SetCard from './SetCard.svelte';

const packs = [
  { id: 1, productType: 'Draft Booster', marketPrice: 5.00, quantity: 3, setName: 'Zendikar', cardsPerPack: 15 },
];

describe('SetCard', () => {
  it('renders set name', () => {
    const { container } = render(SetCard, {
      props: {
        setName: 'Zendikar',
        packs,
        packOdds: { '1': 25.0 },
        multipliers: { '1': 0 },
        onMultChange: vi.fn(),
      },
    });
    expect(container.querySelector('.set-group__name')?.textContent).toBe('Zendikar');
  });

  it('renders one PackRow per pack', () => {
    const { container } = render(SetCard, {
      props: {
        setName: 'Zendikar',
        packs,
        packOdds: { '1': 25.0 },
        multipliers: { '1': 0 },
        onMultChange: vi.fn(),
      },
    });
    expect(container.querySelectorAll('.pack-row')).toHaveLength(1);
  });

  it('renders multiple PackRows', () => {
    const twoPacks = [
      ...packs,
      { id: 2, productType: 'Set Booster', marketPrice: 8.00, quantity: 1, setName: 'Zendikar', cardsPerPack: 15 },
    ];
    const { container } = render(SetCard, {
      props: {
        setName: 'Zendikar',
        packs: twoPacks,
        packOdds: { '1': 60.0, '2': 40.0 },
        multipliers: { '1': 0, '2': 0 },
        onMultChange: vi.fn(),
      },
    });
    expect(container.querySelectorAll('.pack-row')).toHaveLength(2);
  });
});

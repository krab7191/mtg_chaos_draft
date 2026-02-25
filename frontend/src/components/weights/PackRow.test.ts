import { describe, it, expect, vi } from 'vitest';
import { render } from '../../tests/svelte';
import PackRow from './PackRow.svelte';

const pack = { id: 1, productType: 'Draft Booster', marketPrice: 5.00, quantity: 3, cardsPerPack: 15 };

describe('PackRow', () => {
  it('renders productType', () => {
    const { container } = render(PackRow, {
      props: { pack, odds: 25.5, multiplier: 1.0, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.pack-row__type')?.textContent).toBe('Draft Booster');
  });

  it('shows odds percentage', () => {
    const { container } = render(PackRow, {
      props: { pack, odds: 25.5, multiplier: 1.0, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.odds__label')?.textContent).toBe('25.5%');
  });

  it('shows +1.0 for positive multiplier', () => {
    const { container } = render(PackRow, {
      props: { pack, odds: 25.5, multiplier: 1.0, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.weight-val')?.textContent).toBe('+1.0');
  });

  it('shows -0.5 for negative multiplier', () => {
    const { container } = render(PackRow, {
      props: { pack, odds: 25.5, multiplier: -0.5, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.weight-val')?.textContent).toBe('-0.5');
  });

  it('shows quantity', () => {
    const { container } = render(PackRow, {
      props: { pack, odds: 25.5, multiplier: 0, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.pack-row__qty')?.textContent).toBe('3 slots');
  });

  it('shows effective slots with * for non-standard cardsPerPack', () => {
    const nonStandard = { ...pack, quantity: 7, cardsPerPack: 5 };
    const { container } = render(PackRow, {
      props: { pack: nonStandard, odds: 25.5, multiplier: 0, onMultChange: vi.fn() },
    });
    expect(container.querySelector('.pack-row__qty')?.textContent).toBe('2* slots');
  });
});

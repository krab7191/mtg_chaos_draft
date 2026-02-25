import { describe, it, expect, vi } from 'vitest';
import { render } from '../../tests/svelte';
import CollectionRow from './CollectionRow.svelte';

const basePack = { id: 1, productType: 'Draft Booster', marketPrice: 4.99, quantity: 3, cardsPerPack: 15 };

describe('CollectionRow', () => {
  it('renders productType', () => {
    const { container } = render(CollectionRow, { props: { pack: basePack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.pack-row__type')?.textContent).toBe('Draft Booster');
  });

  it('shows quantity', () => {
    const { container } = render(CollectionRow, { props: { pack: basePack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.qty__val')?.textContent).toBe('3');
  });

  it('formats price $4.99', () => {
    const { container } = render(CollectionRow, { props: { pack: basePack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.price-val')?.textContent?.trim()).toBe('$4.99');
  });

  it('shows — for null price', () => {
    const pack = { ...basePack, marketPrice: null };
    const { container } = render(CollectionRow, { props: { pack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.price-val')?.textContent?.trim()).toBe('—');
  });

  it('dec button is disabled at qty 0', () => {
    const pack = { ...basePack, quantity: 0 };
    const { container } = render(CollectionRow, { props: { pack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    const decBtn = container.querySelector('.qty__btn--dec') as HTMLButtonElement;
    expect(decBtn.disabled).toBe(true);
  });

  it('dec button is enabled when qty > 0', () => {
    const { container } = render(CollectionRow, { props: { pack: basePack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    const decBtn = container.querySelector('.qty__btn--dec') as HTMLButtonElement;
    expect(decBtn.disabled).toBe(false);
  });

  it('appends * to qty when cardsPerPack is non-standard', () => {
    const pack = { ...basePack, cardsPerPack: 5 };
    const { container } = render(CollectionRow, { props: { pack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.qty__val')?.textContent).toBe('3 *');
  });

  it('does not append * when cardsPerPack is 15', () => {
    const { container } = render(CollectionRow, { props: { pack: basePack, onQtyChange: vi.fn(), onDelete: vi.fn() } });
    expect(container.querySelector('.qty__val')?.textContent).toBe('3');
  });
});

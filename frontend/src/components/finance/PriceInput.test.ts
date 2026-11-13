import { describe, it, expect } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import PriceInput from './PriceInput.svelte';

describe('PriceInput', () => {
  it('shows the per-pack price for a total', () => {
    const { container } = render(PriceInput, { props: { value: 132.18, mode: 'total', quantity: 30 } });
    expect(container.querySelector('.price__other')?.textContent).toBe('= $4.41 each');
  });

  it('shows the total for a per-pack price, after switching modes', async () => {
    const { container } = render(PriceInput, { props: { value: 4.5, mode: 'total', quantity: 2 } });
    const each = Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Each')!;
    await act(() => fireEvent.click(each));
    expect(each.getAttribute('aria-pressed')).toBe('true');
    expect(container.querySelector('.price__other')?.textContent).toBe('= $9.00 total');
    expect(container.querySelector('input')?.placeholder).toBe('$ per pack');
  });

  it('shows nothing extra for one pack or no price', () => {
    const one = render(PriceInput, { props: { value: 5, mode: 'total', quantity: 1 } });
    expect(one.container.querySelector('.price__other')?.textContent).toBe('');
    const none = render(PriceInput, { props: { value: null, mode: 'each', quantity: 3 } });
    expect(none.container.querySelector('.price__other')?.textContent).toBe('');
  });
});

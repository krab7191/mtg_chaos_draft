import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import PackCheck from './PackCheck.svelte';
import type { Breakdown } from '../../lib/deals';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers(); });

const rna: Breakdown = {
  mtgstocksId: 7, setName: 'Ravnica Allegiance', productType: 'Collector Booster', price: 168, noSales: true,
  value: 12.28, ratio: 13.7, premium: true, cards: 15, pricesDay: '2026-11-19',
  slots: [{ name: 'rare mythic', count: 3, value: 7.09 }, { name: 'foil', count: 2, value: 2.45 }],
  top: [{ name: 'Smothering Tithe', foil: false, price: 52.42, chance: 3 / 55, value: 2.86 },
        { name: 'Rhythm of the Wild', foil: true, price: 4.71, chance: 0.6, value: 2.8 }],
};

function mount(contents: Breakdown | null) {
  const fetchMock = vi.fn(async (url: string): Promise<unknown> => {
    if (url.startsWith('/api/search')) {
      return { ok: true, status: 200, json: async (): Promise<unknown> => [
        { mtgstocksId: 7, setName: 'Ravnica Allegiance', productType: 'Collector Booster', marketPrice: 1875 },
        { mtgstocksId: 8, setName: 'Ravnica Allegiance', productType: 'Booster Pack', marketPrice: 4.99 },
      ] };
    }
    return contents
      ? { ok: true, status: 200, json: async (): Promise<unknown> => contents }
      : { ok: false, status: 404, text: async (): Promise<string> => 'no pull-rate data for this pack' };
  });
  vi.stubGlobal('fetch', fetchMock);
  return { ...render(PackCheck, {}), fetchMock };
}

async function find(container: HTMLElement, q: string) {
  vi.useFakeTimers();
  const input = container.querySelector('input[type=search]') as HTMLInputElement;
  await act(() => fireEvent.input(input, { target: { value: q } }));
  await act(async () => { await vi.advanceTimersByTimeAsync(400); });
  vi.useRealTimers();
}

describe('PackCheck', () => {
  it('finds a pack and breaks down what its contents are worth', async () => {
    const { container, fetchMock } = mount(rna);
    await find(container, 'ravnica allegiance');
    expect(fetchMock).toHaveBeenCalledWith('/api/search?q=ravnica%20allegiance', undefined);
    const options = Array.from(container.querySelectorAll('.results button'));
    expect(options.map(b => b.textContent)).toEqual(['Ravnica Allegiance — Collector Booster', 'Ravnica Allegiance — Booster Pack']);
    await act(() => fireEvent.click(options[0]));
    await waitFor(() => expect(container.querySelector('.summary')).not.toBeNull());
    const text = container.textContent ?? '';
    expect(text).toContain('contents worth $12.28 per pack');
    expect(text).toContain('against a price of $168.00 (asking prices only: no recent sales)');
    expect(text).toContain("⚠ you'd pay 13.7× what the cards inside are worth");
    expect(container.querySelector('.summary')?.classList.contains('premium')).toBe(true);
    expect(text).toContain('3 × rare mythic');
    expect(text).toContain('Smothering Tithe');
    expect(text).toContain('1 in 18 packs');
    expect(text).toContain('Rhythm of the Wild (foil)');
    expect(text).toContain('0.6 per pack');
  });

  it('says so when a pack has no breakdown, and ignores short queries', async () => {
    const { container, fetchMock } = mount(null);
    await find(container, 'r');
    expect(fetchMock).not.toHaveBeenCalled();
    await find(container, 'ravnica');
    await act(() => fireEvent.click(container.querySelectorAll('.results button')[1]));
    await waitFor(() => expect(container.textContent).toContain('No breakdown for Ravnica Allegiance Booster Pack'));
  });

  it('shows a contents value without a price, and a fractional slot count', async () => {
    const { container } = mount({ ...rna, price: undefined, ratio: undefined, premium: false, noSales: false,
      slots: [{ name: 'wildcard', count: 1.5, value: 3 }] });
    await find(container, 'ravnica');
    await act(() => fireEvent.click(container.querySelectorAll('.results button')[0]));
    await waitFor(() => expect(container.querySelector('.summary')).not.toBeNull());
    expect(container.textContent).not.toContain('against a price');
    expect(container.textContent).toContain('1.5 × wildcard');
  });
});

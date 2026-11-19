import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import StoreDeals from './StoreDeals.svelte';
import { toast } from '../../lib/toast.svelte';
import type { OffersResponse, StoreOffer } from '../../lib/deals';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); localStorage.clear(); });

const offer = (id: number, over: Partial<StoreOffer> = {}): StoreOffer => ({
  store: 'Collector Store', url: `https://x/${id}`, mtgstocksId: id, title: `Magic the Gathering: Set ${id} Draft Booster Pack`, price: 40, available: true,
  seenAt: '2026-11-19T00:00:00Z', setName: `Set ${id}`, productType: 'Booster Pack', releasedAt: '2000-06-05',
  marketPrice: 56, discount: 0.29, isDeal: true, ...over,
});

function mount(resp: OffersResponse | null, props = { canEdit: true }, watch: { ok: boolean } = { ok: true }) {
  const fetchMock = vi.fn(async (url: string) => {
    if (url === '/api/store-offers') {
      return resp
        ? { ok: true, status: 200, json: async (): Promise<unknown> => resp }
        : { ok: false, status: 502, text: async (): Promise<string> => 'down' };
    }
    return { ok: watch.ok, status: watch.ok ? 204 : 500, json: async (): Promise<unknown> => ({}), text: async (): Promise<string> => '' };
  });
  vi.stubGlobal('fetch', fetchMock);
  return { ...render(StoreDeals, { props }), fetchMock };
}

const names = (c: HTMLElement) => Array.from(c.querySelectorAll('tbody td.name'))
  .map(td => td.querySelector('.sub')?.textContent?.match(/MTGStocks' (Set \d+)/)?.[1]);
const button = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('button')).find(b => b.textContent?.trim() === label)!;
const checkbox = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('label')).find(l => l.textContent?.trim() === label)!.querySelector('input')!;

describe('StoreDeals', () => {
  const resp: OffersResponse = {
    offers: [
      offer(1, { contents: 31 }),
      offer(2, { isDeal: false, discount: -0.1, price: 61.6, contents: 12, premium: true }),
      offer(3, { isDeal: false, available: false, discount: 0.4, language: 'ja' }),
    ],
    stores: ['Collector Store'], watched: [2], dealDiscount: 0.15,
  };

  it('shows deals first, then in-stock or all listings, with links elsewhere', async () => {
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    expect(container.textContent).toContain('Single packs at Collector Store');
    expect(container.querySelector('tbody')?.textContent).toContain('29% under');
    expect(container.querySelector('tbody')?.textContent).toContain('contents $31.00');
    // $40 for $31 of cards: red. Under the contents would be green.
    expect(Array.from(container.querySelectorAll('tbody .sub')).find(e => e.textContent?.includes('contents $31.00'))?.classList.contains('tone--higher')).toBe(true);
    expect(container.querySelector('tbody a')?.getAttribute('href')).toBe('https://x/1');
    // The store's own name, then what it's priced against.
    expect(container.querySelector('tbody td.name')?.textContent).toContain('Set 1 Draft Booster Pack');
    expect(container.querySelector('tbody td.name')?.textContent).toContain("priced as MTGStocks' Set 1 Booster Pack · 2000");
    expect(Array.from(container.querySelectorAll('.elsewhere a')).map(a => a.textContent)).toContain('Card Kingdom');

    await act(() => fireEvent.click(button(container, 'In stock')));
    expect(names(container)).toEqual(['Set 1']); // Set 2 is above market: hidden by default
    await act(() => fireEvent.click(checkbox(container, 'Show listings above market')));
    expect(names(container)).toEqual(['Set 1', 'Set 2']);
    expect(container.querySelector('tbody')?.textContent).toContain('10% over');
    expect(container.querySelector('tbody')?.textContent).toContain('⚠ 5.1× contents');
    await act(() => fireEvent.click(button(container, 'All listings')));
    expect(names(container)).toEqual(['Set 1', 'Set 2', 'Set 3']);
    expect(container.textContent).toContain('Sold out');
    expect(container.querySelector('.lang')?.textContent).toBe('Japanese');
    await act(() => fireEvent.click(button(container, 'Deals (1)')));
    expect(names(container)).toEqual(['Set 1']);
  });

  it('opens on in-stock listings when there are no deals', async () => {
    const { container } = mount({ ...resp, offers: [offer(2, { isDeal: false, discount: 0 })] });
    await waitFor(() => expect(names(container)).toEqual(['Set 2']));
    expect(container.querySelector('tbody')?.textContent).toContain('at market');
    await act(() => fireEvent.click(button(container, 'Deals (0)')));
    expect(container.textContent).toContain('No deals right now with these filters');
  });

  it('watches a pack from its listing, and snaps back if saving fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container, fetchMock } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    const box = container.querySelector('tbody input[type=checkbox]') as HTMLInputElement;
    expect(box.checked).toBe(false);
    await act(() => fireEvent.click(box));
    await waitFor(() => expect(show).toHaveBeenCalledWith(expect.stringContaining('Watching Set 1 Booster Pack'), 'success'));
    expect(fetchMock).toHaveBeenCalledWith('/api/trends/1/watch', expect.objectContaining({ method: 'PUT' }));
    expect(box.checked).toBe(true);
    await act(() => fireEvent.click(box)); // unwatch
    await waitFor(() => expect(box.checked).toBe(false));
  });

  it('keeps the box unticked when watching fails', async () => {
    const { container } = mount(resp, { canEdit: true }, { ok: false });
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    const box = container.querySelector('tbody input[type=checkbox]') as HTMLInputElement;
    await act(() => fireEvent.click(box));
    await waitFor(() => expect(box.checked).toBe(false));
  });

  it('viewers see no watch boxes; empty and failed loads say so', async () => {
    const viewer = mount(resp, { canEdit: false });
    await waitFor(() => expect(names(viewer.container)).toEqual(['Set 1']));
    expect(viewer.container.querySelector('tbody input')).toBeNull();
    viewer.unmount();

    const empty = mount({ ...resp, offers: [], stores: [] });
    await waitFor(() => expect(empty.container.textContent).toContain('No store listings yet'));
    expect(empty.container.textContent).toContain('Single packs at online stores');
    empty.unmount();

    const failed = mount(null);
    await waitFor(() => expect(failed.container.textContent).toContain("Store listings aren't available right now"));
  });
});

describe('StoreDeals listing names', () => {
  it("drops CardTrader's language and seller suffix, and falls back to the product without a title", async () => {
    const resp: OffersResponse = {
      offers: [
        offer(1, { store: 'CardTrader', title: "Dragon's Maze Booster · Japanese · ships from BE", language: 'ja' }),
        offer(2, { title: '' }),
      ],
      stores: ['CardTrader'], watched: [], dealDiscount: 0.15,
    };
    const { container } = mount(resp);
    await waitFor(() => expect(container.querySelectorAll('tbody td.name')).toHaveLength(2));
    const cells = Array.from(container.querySelectorAll('tbody td.name')).map(td => td.textContent ?? '');
    expect(cells[0]).toContain("Dragon's Maze Booster");
    expect(cells[0]).not.toContain('ships from');
    expect(cells[1]).toContain('Set 2 Booster Pack');
  });
});

describe('StoreDeals filters', () => {
  const resp: OffersResponse = {
    offers: [
      offer(1, { price: 4, contents: 6 }),
      offer(2, { price: 30, language: 'it' }),
      offer(3, { price: 8, language: 'ja' }),
    ],
    stores: ['CardTrader'], watched: [], dealDiscount: 0.15,
  };

  it('filters by language and max price, and remembers the choice', async () => {
    const { container, unmount } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 1', 'Set 2', 'Set 3']));
    expect(Array.from(container.querySelectorAll('.langs label')).map(l => l.textContent?.trim()))
      .toEqual(['English', 'Italian', 'Japanese']);
    expect(Array.from(container.querySelectorAll('tbody .sub')).find(e => e.textContent?.includes('contents $6.00'))?.classList.contains('tone--great')).toBe(true);
    await act(() => fireEvent.click(checkbox(container, 'Japanese')));
    expect(names(container)).toEqual(['Set 1', 'Set 2']);
    const max = container.querySelector('.maxprice input') as HTMLInputElement;
    await act(() => fireEvent.change(max, { target: { value: '10' } }));
    expect(names(container)).toEqual(['Set 1']);
    expect(container.textContent).toContain('Deals (1)');
    unmount();

    // Next visit: same filters.
    const again = mount(resp);
    await waitFor(() => expect(names(again.container)).toEqual(['Set 1']));
    expect((again.container.querySelector('.maxprice input') as HTMLInputElement).value).toBe('10');
    // Clearing the price and ticking Japanese again shows everything.
    await act(() => fireEvent.change(again.container.querySelector('.maxprice input')!, { target: { value: '' } }));
    await act(() => fireEvent.click(checkbox(again.container, 'Japanese')));
    expect(names(again.container)).toEqual(['Set 1', 'Set 2', 'Set 3']);
  });

  it('works without browser storage', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 1', 'Set 2', 'Set 3']));
    await act(() => fireEvent.click(checkbox(container, 'English')));
    expect(names(container)).toEqual(['Set 2', 'Set 3']);
  });
});

describe('StoreDeals grouping', () => {
  it('shows one row per pack with its other listings on demand, and the first 25 packs', async () => {
    const many = Array.from({ length: 30 }, (_, i) => offer(i + 1));
    const resp: OffersResponse = {
      offers: [
        offer(1, { store: 'Card Kingdom', url: 'https://ck/1', price: 30 }),
        offer(1, { store: 'CardTrader', url: 'https://ct/1', price: 33, language: 'ja', discount: 0.2 }),
        ...many.slice(1),
      ],
      stores: ['Card Kingdom', 'CardTrader'], watched: [], dealDiscount: 0.15,
    };
    const { container } = mount(resp);
    await waitFor(() => expect(container.querySelectorAll('tbody td.name')).toHaveLength(25));
    expect(container.textContent).toContain('Deals (30)'); // packs, not listings
    const more = Array.from(container.querySelectorAll('.more')).find(b => b.textContent?.includes('+1 more listing'))!;
    await act(() => fireEvent.click(more));
    expect(container.querySelector('.others')?.textContent).toContain('CardTrader');
    expect(container.querySelector('.others')?.textContent).toContain('Japanese');
    expect(container.querySelector('.others')?.textContent).toContain('20% under');
    await act(() => fireEvent.click(container.querySelector('.more')!)); // hide again
    expect(container.querySelector('.others')).toBeNull();
    await act(() => fireEvent.click(button(container, 'Show all 30')));
    expect(container.querySelectorAll('tbody td.name')).toHaveLength(30);
  });
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import BestDeals from './BestDeals.svelte';
import type { BestDealsResponse, Candidate } from '../../lib/bestdeals';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); localStorage.clear(); });

const cand = (id: number, over: Partial<Candidate> = {}): Candidate => ({
  mtgstocksId: id, setName: `Set ${id}`, productType: 'Booster Pack', releasedAt: '2007-01-01', store: 'Card Kingdom',
  url: `https://ck/${id}`, title: 't', language: 'en', price: 10, market: 12, contents: 30 - id, signal: 'stable', timing: 1, ...over,
});

function mount(resp: BestDealsResponse | null) {
  vi.stubGlobal('fetch', vi.fn(async (): Promise<unknown> => resp
    ? { ok: true, status: 200, json: async (): Promise<unknown> => resp }
    : { ok: false, status: 502, text: async (): Promise<string> => 'down' }));
  return render(BestDeals, {});
}

const names = (c: HTMLElement) => Array.from(c.querySelectorAll('.pick__name')).map(h => h.textContent?.replace(/\s+/g, ' ').trim());

describe('BestDeals', () => {
  const resp: BestDealsResponse = {
    candidates: [cand(1, { language: 'fr', store: 'CardTrader' }), cand(2), cand(3), cand(4), cand(5)],
    shipping: { 'Card Kingdom': 7, CardTrader: 5, TCGplayer: 4 }, asOf: '2026-11-19',
  };

  it('shows the top 3 with their reasons, and runners-up', async () => {
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 1 Booster Pack French', 'Set 2 Booster Pack', 'Set 3 Booster Pack']));
    expect(container.querySelector('.pick__why')?.textContent).toContain('of cards per $1');
    expect(container.querySelector('.pick__buy a')?.getAttribute('href')).toBe('https://ck/1');
    expect(container.querySelector('.runners')?.textContent).toContain('Set 4');
    expect(container.textContent).toContain('all languages · any price');
  });

  it('follows the Cheap Packs language filter: no French means the next pack moves up', async () => {
    localStorage.setItem('storeListingFilters', JSON.stringify({ languages: ['en'], maxPrice: 50, showOver: false }));
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toEqual(['Set 2 Booster Pack', 'Set 3 Booster Pack', 'Set 4 Booster Pack']));
    expect(container.textContent).toContain('languages: English · up to $50.00');
  });

  it('shipping settings change the delivered price and are remembered', async () => {
    const { container, unmount } = mount(resp);
    await waitFor(() => expect(names(container)).toHaveLength(3));
    expect(container.querySelector('.pick__buy')?.textContent).toContain('about $11.67 with shipping'); // $10 + $5 / 3
    const inputs = container.querySelectorAll('.ship input');
    await act(() => fireEvent.change(inputs[1], { target: { value: '0' } })); // CardTrader
    await act(() => fireEvent.change(inputs[3], { target: { value: '1' } })); // packs per order
    expect(container.querySelector('.pick__buy')?.textContent).toContain('about $10.00 with shipping');
    unmount();
    const again = mount(resp);
    await waitFor(() => expect(names(again.container)).toHaveLength(3));
    expect((again.container.querySelectorAll('.ship input')[3] as HTMLInputElement).value).toBe('1');
    await act(() => fireEvent.change(again.container.querySelectorAll('.ship input')[0], { target: { value: 'x' } }));
    await act(() => fireEvent.change(again.container.querySelectorAll('.ship input')[3], { target: { value: '0' } }));
    expect((again.container.querySelectorAll('.ship input')[3] as HTMLInputElement).value).toBe('1');
  });

  it('says so when there is nothing to rank or the API is down', async () => {
    const empty = mount({ ...resp, candidates: [] });
    await waitFor(() => expect(empty.container.textContent).toContain('Nothing to rank yet'));
    empty.unmount();
    const down = mount(null);
    await waitFor(() => expect(down.container.textContent).toContain("aren't available right now"));
  });

  it('copes with blocked browser storage', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toHaveLength(3));
  });
});

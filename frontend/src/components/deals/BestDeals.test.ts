import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import BestDeals from './BestDeals.svelte';
import type { BestDealsResponse, Candidate } from '../../lib/bestdeals';
import { clearFilters, setLanguage, setMaxPrice, ALL_LANGUAGES } from '../../lib/filters.svelte';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); clearFilters(); localStorage.clear(); });

const cand = (id: number, over: Partial<Candidate> = {}): Candidate => ({
  mtgstocksId: id, setName: `Set ${id}`, productType: 'Booster Pack', releasedAt: '2007-01-01', store: 'Card Kingdom',
  url: `https://ck/${id}`, title: 't', language: 'en', price: 10, market: 12, contents: 30 - id, signal: 'stable', timing: 1, ...over,
});

/** Every live-store listing still there, unchanged. */
const echo = (r: BestDealsResponse) => r.candidates.filter(c => c.store === 'CardTrader' || c.store === 'Card Kingdom')
  .map(c => ({ mtgstocksId: c.mtgstocksId, store: c.store, url: c.url, title: c.title, language: c.language, price: c.price }));

/** live: the stores' current listings (default: unchanged; null = unreachable). */
function mount(resp: BestDealsResponse | null, live?: unknown[] | null) {
  if (live === undefined) live = resp ? echo(resp) : [];
  const fetchMock = vi.fn(async (url: string): Promise<unknown> => {
    if (url.startsWith('/api/best-deals/live')) {
      return live ? { ok: true, status: 200, json: async (): Promise<unknown> => ({ offers: live }) } : { ok: false, status: 502 };
    }
    return resp
      ? { ok: true, status: 200, json: async (): Promise<unknown> => resp }
      : { ok: false, status: 502, text: async (): Promise<string> => 'down' };
  });
  vi.stubGlobal('fetch', fetchMock);
  return { ...render(BestDeals, {}), fetchMock };
}

// Set 1's French CardTrader listing, still there when checked live.
const stillThere = undefined; // every listing unchanged (the default)

const names = (c: HTMLElement) => Array.from(c.querySelectorAll('.pick__name')).map(h => h.textContent?.replace(/\s+/g, ' ').trim());

describe('BestDeals', () => {
  const resp: BestDealsResponse = {
    candidates: [cand(1, { language: 'fr', store: 'CardTrader' }), cand(2), cand(3), cand(4), cand(5)],
    shipping: { 'Card Kingdom': 7, CardTrader: 5, TCGplayer: 4 }, asOf: '2026-11-19',
  };

  it('shows the top 3 with their reasons, and runners-up', async () => {
    // Live check: Set 1's French CardTrader listing is still there at the same price.
    const { container, fetchMock } = mount(resp);
    await waitFor(() => expect(container.textContent).toContain('prices checked live'));
    expect(fetchMock).toHaveBeenCalledWith('/api/best-deals/live?ids=1,2,3,4,5');
    expect(names(container)).toEqual(['Set 1 Booster Pack French', 'Set 2 Booster Pack', 'Set 3 Booster Pack']);
    expect(container.querySelector('.pick__why')?.textContent).toContain('of cards per $1');
    expect(container.querySelector('.pick__buy a')?.getAttribute('href')).toBe('https://ck/1');
    expect(container.querySelector('.runners')?.textContent).toContain('Set 4');
  });

  it('follows the shared filters: no French means the next pack moves up, live', async () => {
    const { container } = mount(resp);
    await waitFor(() => expect(names(container)).toHaveLength(3));
    for (const l of ALL_LANGUAGES) if (l !== 'en') setLanguage(l, false);
    await waitFor(() => expect(names(container)).toEqual(['Set 2 Booster Pack', 'Set 3 Booster Pack', 'Set 4 Booster Pack']));
    setMaxPrice(5); // nothing that cheap
    await waitFor(() => expect(names(container)).toEqual([]));
  });

  it('shipping settings change the delivered price and are remembered', async () => {
    const { container, unmount } = mount(resp, stillThere);
    await waitFor(() => expect(names(container)).toHaveLength(3));
    expect(container.querySelector('.pick__buy')?.textContent).toContain('about $11.67 with shipping'); // $10 + $5 / 3
    const inputs = container.querySelectorAll('.ship input');
    await act(() => fireEvent.change(inputs[1], { target: { value: '0' } })); // CardTrader
    await act(() => fireEvent.change(inputs[3], { target: { value: '1' } })); // packs per order
    expect(container.querySelector('.pick__buy')?.textContent).toContain('about $10.00 with shipping');
    unmount();
    const again = mount(resp, stillThere);
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

describe('BestDeals live check', () => {
  const resp: BestDealsResponse = {
    candidates: [cand(1, { store: 'CardTrader', price: 5 }), cand(2), cand(3), cand(4)],
    shipping: { 'Card Kingdom': 0, CardTrader: 0, TCGplayer: 0 }, asOf: '2026-11-19',
  };

  it('re-ranks with live CardTrader prices: a sold-out cheap listing drops out', async () => {
    const { container } = mount(resp, echo(resp).filter(o => o.mtgstocksId !== 1)); // Set 1's cheap CardTrader listing is gone
    await waitFor(() => expect(container.textContent).toContain('prices checked live'));
    expect(names(container)).toEqual(['Set 2 Booster Pack', 'Set 3 Booster Pack', 'Set 4 Booster Pack']);
  });

  it('checks again when a filter brings unchecked CardTrader packs into the top 10', async () => {
    const mixed: BestDealsResponse = {
      // 10 foreign CardTrader packs on top, then English ones further down.
      candidates: [
        ...Array.from({ length: 10 }, (_, i) => cand(i + 1, { store: 'CardTrader', language: 'ja', contents: 50 - i })),
        cand(11, { store: 'CardTrader', contents: 20 }), cand(12, { contents: 19 }), cand(13, { contents: 18 }),
      ],
      shipping: { 'Card Kingdom': 0, CardTrader: 0, TCGplayer: 0 }, asOf: '2026-11-19',
    };
    const liveAll = mixed.candidates.filter(c => c.store === 'CardTrader')
      .map(c => ({ mtgstocksId: c.mtgstocksId, store: 'CardTrader', url: c.url, title: 't', language: c.language, price: c.price }));
    const { container, fetchMock } = mount(mixed, liveAll);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/best-deals/live?ids=1,2,3,4,5,6,7,8,9,10'));
    await waitFor(() => expect(container.textContent).toContain('prices checked live'));
    for (const l of ALL_LANGUAGES) if (l !== 'en') setLanguage(l, false);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/best-deals/live?ids=11,12,13'));
    await waitFor(() => expect(names(container)).toEqual(['Set 11 Booster Pack', 'Set 12 Booster Pack', 'Set 13 Booster Pack']));
  });

  it("says so when CardTrader can't be reached, and keeps the last read", async () => {
    const { container } = mount(resp, null);
    await waitFor(() => expect(container.textContent).toContain("Couldn't reach the stores just now"));
    expect(names(container)[0]).toBe('Set 1 Booster Pack');
  });
});

describe('BestDeals keeps prices fresh', () => {
  const resp: BestDealsResponse = {
    candidates: [cand(1, { store: 'CardTrader' }), cand(2), cand(3)],
    shipping: { 'Card Kingdom': 0, CardTrader: 0, TCGplayer: 0 }, asOf: '2026-11-19',
  };
  afterEach(() => vi.useRealTimers());

  it('retries 2 minutes after a failed check, and re-checks prices once 10 minutes old', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let fail = true;
    const fetchMock = vi.fn(async (url: string): Promise<unknown> => {
      if (url.startsWith('/api/best-deals/live')) {
        return fail ? { ok: false, status: 502 } : { ok: true, status: 200, json: async (): Promise<unknown> => ({ offers: echo(resp) }) };
      }
      return { ok: true, status: 200, json: async (): Promise<unknown> => resp };
    });
    vi.stubGlobal('fetch', fetchMock);
    const { container } = render(BestDeals, {});
    const liveCalls = () => fetchMock.mock.calls.filter(c => String(c[0]).startsWith('/api/best-deals/live')).length;
    await waitFor(() => expect(container.textContent).toContain("Couldn't reach the stores just now"));
    expect(liveCalls()).toBe(1);
    fail = false;
    await act(async () => { await vi.advanceTimersByTimeAsync(2 * 60_000 + 1000); });
    await waitFor(() => expect(container.textContent).toContain('prices checked live'));
    expect(liveCalls()).toBe(2);
    await act(async () => { await vi.advanceTimersByTimeAsync(10 * 60_000 + 1000); });
    await waitFor(() => expect(liveCalls()).toBe(3));
  });
});

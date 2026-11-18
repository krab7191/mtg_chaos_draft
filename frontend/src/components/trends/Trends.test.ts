import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import Trends from './Trends.svelte';
import { toast } from '../../lib/toast.svelte';
import type { TrendRow, TrendsResponse, Trend } from '../../lib/trends';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

const trend = (over: Partial<Trend> = {}): Trend => ({
  current: 120, low: 100, lowDay: '2026-03-01', aboveLow: 0.2, change30: -0.02, releasePrice: 150,
  dropFromRelease: 0.2, band: 0.05, signal: 'stable', spark: [150, 130, 120], sparkStart: '2026-01-01',
  historyDays: 300, released: '2026-01-01', ...over,
});
const row = (id: number, over: Partial<TrendRow> = {}): TrendRow => ({
  mtgstocksId: id, name: 'Play Booster Display', setName: `Set ${id}`, setCode: 'S', releasedAt: '2026-01-01',
  productType: 'Play Booster Box', marketPrice: 118, packsPerBox: 36, trend: trend(), watch: null, alerts: [], ...over,
});
const thresholds = { newSetDays: 45, trendWindow: 30, trendMove: 0.05, floorMinAge: 150, floorDrop: 0.1, floorWindow: 14, floorBand: 0.12, floorNear: 0.1 };
const response = (products: TrendRow[], over: Partial<TrendsResponse> = {}): TrendsResponse =>
  ({ products, notifications: false, backfilled: 10, tracked: 10, thresholds, ...over });

const ok = (body: unknown, status = 200) => ({ ok: true, status, json: async () => body, text: async () => '' });

/** Route fetches by URL; returns the mock so calls can be inspected. */
function api(routes: Record<string, unknown>) {
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    const key = Object.keys(routes).find(k => url.startsWith(k));
    if (!key) throw new Error(`unexpected fetch ${url}`);
    const body = routes[key];
    return typeof body === 'function' ? (body as (u: string, i?: RequestInit) => unknown)(url, init) : ok(body);
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

async function mount(products: TrendRow[], props: Record<string, unknown> = { canEdit: true }, extra: Record<string, unknown> = {}) {
  const fetchMock = api({ '/api/trends?': response(products), ...extra });
  const r = render(Trends, { props });
  await waitFor(() => expect(r.container.querySelector('table, .note')).not.toBeNull());
  return { ...r, fetchMock };
}
const names = (c: HTMLElement) => Array.from(c.querySelectorAll('tbody td.name .link')).map(b => b.textContent);
const button = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('button')).find(b => b.textContent?.includes(label)) as HTMLButtonElement;

describe('Trends page', () => {
  it('shows boxes by default with per-pack prices, and switches to single packs', async () => {
    const { container } = await mount([row(1), row(2, { packsPerBox: undefined, productType: 'Play Booster', trend: trend({ current: 4.5 }) })]);
    expect(names(container)).toEqual(['Set 1']);
    expect(container.textContent).toContain('$3.33 / pack · 36 packs');
    await act(() => fireEvent.click(button(container, 'Single packs')));
    expect(names(container)).toEqual(['Set 2']);
    expect(container.textContent).not.toContain('/ pack');
  });

  it('shows signals with an icon and a label, never color alone', async () => {
    const { container } = await mount([row(1, { trend: trend({ signal: 'floor' }) }), row(2, { trend: null })]);
    const pill = container.querySelector('.signal--floor')!;
    expect(pill.textContent).toContain('◆');
    expect(pill.textContent).toContain('Possible floor');
    expect(container.textContent).toContain('no history yet');
  });

  it('filters to possible floors and to watched products', async () => {
    const { container } = await mount([
      row(1, { trend: trend({ signal: 'floor' }) }),
      row(2, { watch: { mtgstocksId: 2, dropAlert: null } }),
      row(3),
    ]);
    await act(() => fireEvent.click(button(container, 'Possible floors')));
    expect(names(container)).toEqual(['Set 1']);
    await act(() => fireEvent.click(button(container, 'Watched')));
    expect(names(container)).toEqual(['Set 2']);
    await act(() => fireEvent.click(button(container, 'All')));
    expect(names(container)).toHaveLength(3);
  });

  it('reloads for a different time range', async () => {
    const { container, fetchMock } = await mount([row(1)]);
    const select = container.querySelector('select')!;
    await act(() => fireEvent.change(select, { target: { value: '5' } }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/trends?years=5', undefined));
  });

  it('opens a row to show its price chart, loading history once', async () => {
    const { container, fetchMock } = await mount([row(1)], { canEdit: true }, {
      '/api/trends/1/history': [{ day: '2026-01-01', price: 150 }, { day: '2026-02-01', price: 120 }],
    });
    await act(() => fireEvent.click(button(container, 'Set 1')));
    await waitFor(() => expect(container.querySelector('figure.chart')).not.toBeNull());
    expect(container.querySelector('.chart__title')?.textContent).toContain('Set 1 Play Booster Box');
    await act(() => fireEvent.click(button(container, 'Set 1'))); // close
    expect(container.querySelector('figure.chart')).toBeNull();
    await act(() => fireEvent.click(button(container, 'Set 1'))); // reopen, cached
    expect(fetchMock.mock.calls.filter(c => String(c[0]).includes('/history'))).toHaveLength(1);
  });

  it('watches a product, sets a drop alert, and raises the alert', async () => {
    const puts: unknown[] = [];
    const { container } = await mount([row(1, { trend: trend({ dropFromRelease: 0.35 }) })], { canEdit: true }, {
      '/api/trends/1/watch': (_u: string, init?: RequestInit) => { puts.push(JSON.parse(String(init?.body))); return ok(null, 204); },
    });
    const box = container.querySelector<HTMLInputElement>('.watch input[type=checkbox]')!;
    await act(() => fireEvent.change(box, { target: { checked: true } }));
    await waitFor(() => expect(container.querySelector('.watch__drop')).not.toBeNull());
    expect(puts[0]).toEqual({ watch: true, dropAlert: null });

    const drop = container.querySelector<HTMLInputElement>('.watch__drop input')!;
    await act(() => fireEvent.change(drop, { target: { value: '30' } }));
    await waitFor(() => expect(container.querySelector('.alerts')).not.toBeNull());
    expect(puts[1]).toEqual({ watch: true, dropAlert: 0.3 });
    expect(container.querySelector('.alerts')?.textContent).toContain('past your −30% alert');

    // Out-of-range input is ignored; clearing removes the threshold.
    await act(() => fireEvent.change(drop, { target: { value: '150' } }));
    expect(puts).toHaveLength(2);
    await act(() => fireEvent.change(drop, { target: { value: '' } }));
    await waitFor(() => expect(puts).toHaveLength(3));
    expect(container.querySelector('.alerts')).toBeNull();

    await act(() => fireEvent.change(box, { target: { checked: false } }));
    await waitFor(() => expect(container.querySelector('.watch__drop')).toBeNull());
  });

  it('a watched floor alerts, and clicking it opens that row on the right tab', async () => {
    const floorPack = row(2, { packsPerBox: undefined, productType: 'Play Booster', trend: trend({ signal: 'floor', current: 4 }),
      watch: { mtgstocksId: 2, dropAlert: null }, alerts: ['floor'] });
    const { container } = await mount([row(1), floorPack], { canEdit: true }, { '/api/trends/2/history': [] });
    expect(container.querySelector('.alerts')?.textContent).toContain('1 watched product needs a look');
    expect(container.querySelector('.alerts')?.textContent).toContain('possible floor at $4.00');
    await act(() => fireEvent.click(container.querySelector('.alerts .link')!));
    expect(names(container)).toEqual(['Set 2']);
    await waitFor(() => expect(container.textContent).toContain('Not enough price history'));
  });

  it('keeps the watch unchanged when saving fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = await mount([row(1)], { canEdit: true }, {
      '/api/trends/1/watch': { ok: false, status: 500, text: async () => '' },
    });
    // The route returns a plain object, so wrap it as a failed response.
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 500, text: async () => '' })));
    await act(() => fireEvent.change(container.querySelector('.watch input')!, { target: { checked: true } }));
    await waitFor(() => expect(show).toHaveBeenCalledWith(expect.stringContaining("Couldn't update the watch"), 'error'));
    expect(container.querySelector('.watch__drop')).toBeNull();
    await waitFor(() => expect(container.querySelector<HTMLInputElement>('.watch input')!.checked).toBe(false));
  });

  it('hides watch controls from viewers and explains a running backfill', async () => {
    api({ '/api/trends?': response([row(1)], { backfilled: 3, tracked: 10 }) });
    const { container } = render(Trends, { props: {} });
    await waitFor(() => expect(container.querySelector('table')).not.toBeNull());
    expect(container.querySelector('.watch')).toBeNull();
    expect(container.querySelector('.history')?.textContent).toBe('History 3/10');
    expect(container.querySelector('.explain')?.textContent?.replace(/\s+/g, ' ')).toContain('released at least 150 days ago, at least 10% below its release price');
  });

  it('says so when nothing matches, and survives a failed load', async () => {
    const empty = await mount([]);
    expect(empty.container.textContent).toContain('Nothing matches');
    empty.unmount();

    const show = vi.spyOn(toast, 'show');
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 502, text: async () => "couldn't reach MTGStocks" })));
    const failed = render(Trends, { props: {} });
    await waitFor(() => expect(show).toHaveBeenCalledWith("Couldn't load price trends: couldn't reach MTGStocks", 'error'));
    expect(failed.container.textContent).toContain('Nothing matches');
  });

  it('watching a product already at a floor alerts right away; "at the low" is spelled out', async () => {
    const atFloor = row(1, { trend: trend({ signal: 'floor', aboveLow: 0, current: 100 }) });
    const { container } = await mount([atFloor], { canEdit: true }, { '/api/trends/1/watch': ok(null, 204) });
    expect(container.textContent).toContain('at the low');
    await act(() => fireEvent.change(container.querySelector('.watch input')!, { target: { checked: true } }));
    await waitFor(() => expect(container.querySelector('.alerts')?.textContent).toContain('possible floor at $100.00'));
    // Its alert link opens the box tab.
    await act(() => fireEvent.click(button(container, 'Single packs')));
    await act(() => fireEvent.click(container.querySelector('.alerts .link')!));
    expect(names(container)).toEqual(['Set 1']);
  });

  it('keeps a row open with a loading note when its history fails to load', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = await mount([row(1)], { canEdit: true }, {
      '/api/trends/1/history': () => ({ ok: false, status: 500, text: async () => '' }),
    });
    await act(() => fireEvent.click(button(container, 'Set 1')));
    await waitFor(() => expect(show).toHaveBeenCalledWith(expect.stringContaining("Couldn't load price history"), 'error'));
    expect(container.querySelector('tr.detail')?.textContent).toContain('Loading history');
  });

  it('watching gives immediate feedback on whether it would alert', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = await mount(
      [row(1, { trend: trend({ signal: 'floor', current: 90, dropFromRelease: 0.4 }) }), row(2, { trend: trend({ signal: 'falling' }) })],
      { canEdit: true },
      { '/api/trends/1/watch': ok(null, 204), '/api/trends/2/watch': ok(null, 204) },
    );
    const boxes = container.querySelectorAll<HTMLInputElement>('tbody .watch input[type=checkbox]');
    await act(() => fireEvent.change(boxes[0], { target: { checked: true } }));
    await waitFor(() => expect(show).toHaveBeenCalledWith(
      'Watching Set 1 Play Booster Box: alert now, possible floor at $90.00, −40% since release', 'success'));
    await act(() => fireEvent.change(boxes[1], { target: { checked: true } }));
    await waitFor(() => expect(show).toHaveBeenCalledWith('Watching Set 2 Play Booster Box. No alert right now (falling).', 'info'));
  });

  it('a drop alert that starts firing says so', async () => {
    const show = vi.spyOn(toast, 'show');
    const watched = row(1, { trend: trend({ dropFromRelease: 0.35 }), watch: { mtgstocksId: 1, dropAlert: null } });
    const { container } = await mount([watched], { canEdit: true }, { '/api/trends/1/watch': ok(null, 204) });
    await act(() => fireEvent.change(container.querySelector('.watch__drop input')!, { target: { value: '30' } }));
    await waitFor(() => expect(show).toHaveBeenCalledWith(expect.stringContaining('alert now, −35% since release, past your −30% alert'), 'success'));
  });

  it('watches and unwatches every row shown at once, with a summary', async () => {
    const show = vi.spyOn(toast, 'show');
    const bodies: unknown[] = [];
    const { container } = await mount(
      [row(1, { trend: trend({ signal: 'floor' }) }), row(2, { watch: { mtgstocksId: 2, dropAlert: 0.5 } }), row(3),
       row(4, { packsPerBox: undefined, productType: 'Play Booster' })],
      { canEdit: true },
      { '/api/trends/watch': (_u: string, init?: RequestInit) => { bodies.push(JSON.parse(String(init?.body))); return ok(null, 204); } },
    );
    const all = container.querySelector<HTMLInputElement>('thead .watch__all input')!;
    expect(all.indeterminate).toBe(true); // one of the three boxes is watched

    await act(() => fireEvent.change(all, { target: { checked: true } }));
    await waitFor(() => expect(bodies).toHaveLength(1));
    // Only the unwatched boxes shown are sent; the single pack (other tab) isn't touched.
    expect(bodies[0]).toEqual({ ids: [1, 3], watch: true });
    expect(show).toHaveBeenCalledWith('Watching 2 · 1 already alerting (see the top of the page)', 'success');
    expect(all.checked).toBe(true);
    expect(all.indeterminate).toBe(false);
    // The existing drop alert survives.
    expect(container.querySelectorAll<HTMLInputElement>('.watch__drop input')[1].value).toBe('50');

    await act(() => fireEvent.change(all, { target: { checked: false } }));
    await waitFor(() => expect(bodies).toHaveLength(2));
    expect(bodies[1]).toEqual({ ids: [1, 2, 3], watch: false });
    expect(show).toHaveBeenLastCalledWith('Stopped watching 3', 'info');
    expect(container.querySelector('.alerts')).toBeNull();
  });

  it('select all says when nothing is alerting, and does nothing on an empty list or failure', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container, fetchMock } = await mount([row(1)], { canEdit: true }, { '/api/trends/watch': ok(null, 204) });
    const all = container.querySelector<HTMLInputElement>('thead .watch__all input')!;
    await act(() => fireEvent.change(all, { target: { checked: true } }));
    await waitFor(() => expect(show).toHaveBeenCalledWith('Watching 1 · none alerting right now', 'info'));

    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 500, text: async () => '' })));
    await act(() => fireEvent.change(all, { target: { checked: false } }));
    await waitFor(() => expect(show).toHaveBeenLastCalledWith(expect.stringContaining("Couldn't update the watches"), 'error'));
    await waitFor(() => expect(all.checked).toBe(true)); // snapped back: still watched
    expect(fetchMock).toBeDefined();

    // Filtering to nothing leaves select-all with nothing to do.
    await act(() => fireEvent.click(button(container, 'Possible floors')));
    expect(container.querySelector('thead')).toBeNull();
  });

  it('explains how to turn on phone alerts, and sends a test alert once they are on', async () => {
    const off = await mount([row(1)]);
    expect(off.container.querySelector('.phone')?.textContent).toContain('Phone alerts are off');
    off.unmount();

    const show = vi.spyOn(toast, 'show');
    let calls = 0;
    api({
      '/api/trends?': response([row(1)], { notifications: true }),
      '/api/trends/notify-test': () => (++calls === 1 ? ok(null, 204) : { ok: false, status: 400, text: async () => 'not set up' }),
    });
    const on = render(Trends, { props: { canEdit: true } });
    await waitFor(() => expect(on.container.querySelector('.phone')?.textContent).toContain('Phone alerts are on'));
    await act(() => fireEvent.click(button(on.container, 'Send test alert')));
    await waitFor(() => expect(show).toHaveBeenCalledWith('Test alert sent: check your phone', 'success'));
    await act(() => fireEvent.click(button(on.container, 'Send test alert')));
    await waitFor(() => expect(show).toHaveBeenLastCalledWith('Test alert failed: not set up', 'error'));
  });

  it('viewers see no phone-alert controls', async () => {
    api({ '/api/trends?': response([row(1)], { notifications: true }) });
    const { container } = render(Trends, { props: {} });
    await waitFor(() => expect(container.querySelector('table')).not.toBeNull());
    expect(container.querySelector('.phone')).toBeNull();
  });

  it('notes low or no stock on alerts, and nothing when well stocked', async () => {
    const alerting = (id: number, stock: number) => row(id, {
      setName: `Set ${id}`, stock, trend: trend({ signal: 'floor' }), watch: { mtgstocksId: id, dropAlert: null }, alerts: ['floor'],
    });
    const { container } = await mount([alerting(1, 0), alerting(2, 6), alerting(3, 24)]);
    const items = Array.from(container.querySelectorAll('.alerts li')).map(li => li.querySelector('.stock-note')?.textContent ?? null);
    expect(items).toEqual(['none in stock', '6 in stock', null]);
  });
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import CheapestPacks from './CheapestPacks.svelte';
import { toast } from '../../lib/toast.svelte';
import type { Trend, TrendRow } from '../../lib/trends';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

const trend = (over: Partial<Trend> = {}): Trend => ({
  current: 4, low: 3, lowDay: '2015-01-01', aboveLow: 0.3, change30: -0.05, releasePrice: 4, dropFromRelease: 0,
  band: 0.05, signal: 'stable', spark: [5, 4.5, 4], sparkStart: '2013-05-03', historyDays: 3000, released: '2013-05-03',
  usual: 5, vsUsual: -0.2, dip: true, ...over,
});
const row = (id: number, over: Partial<TrendRow> = {}): TrendRow => ({
  mtgstocksId: id, name: 'Booster Pack', setName: `Set ${id}`, setCode: 'S', releasedAt: '2013-05-03',
  productType: 'Booster Pack', marketPrice: 4, trend: trend(), watch: null, alerts: [], ...over,
});

function mount(products: TrendRow[] | null, props: Record<string, unknown> = { canEdit: true }, watchOk = true) {
  const fetchMock = vi.fn(async (url: string): Promise<unknown> => {
    if (url.startsWith('/api/cheapest/trends')) {
      return products
        ? { ok: true, status: 200, json: async (): Promise<unknown> => ({ products, dipBelowUsual: 0.15, muteAtStock: 20, notifications: true }) }
        : { ok: false, status: 502, text: async (): Promise<string> => 'down' };
    }
    if (url.endsWith('/history')) {
      return { ok: true, status: 200, json: async (): Promise<unknown> => [
        { day: '2026-01-01T00:00:00Z', price: 5 }, { day: '2026-06-01T00:00:00Z', price: 4 },
      ] };
    }
    return { ok: watchOk, status: watchOk ? 204 : 500, json: async (): Promise<unknown> => ({}), text: async (): Promise<string> => '' };
  });
  vi.stubGlobal('fetch', fetchMock);
  return { ...render(CheapestPacks, { props }), fetchMock };
}

const names = (c: HTMLElement) => Array.from(c.querySelectorAll('tbody td.name .link')).map(b => b.textContent);
const button = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('button')).find(b => b.textContent?.trim() === label)!;

describe('CheapestPacks', () => {
  it('lists packs with how they compare to their usual price, and opens a chart', async () => {
    const { container, fetchMock } = mount([
      row(1, { watch: { mtgstocksId: 1, dropAlert: null }, alerts: ['dip'], ev: { value: 4.4, change: 0.08, day: '2026-11-19' } }),
      row(2, { trend: trend({ vsUsual: 0.05, dip: false }) }),
      row(3, { trend: trend({ vsUsual: 0.001, dip: false }) }),
      row(4, { trend: null, marketPrice: 6 }),
    ]);
    await waitFor(() => expect(names(container)).toEqual(['Set 1', 'Set 2', 'Set 3', 'Set 4']));
    expect(fetchMock).toHaveBeenCalledWith('/api/cheapest/trends?before=2020-01-01', undefined);
    const text = (id: number) => container.querySelector(`#cheap-${id}`)?.textContent ?? '';
    expect(text(1)).toContain('20% under usual');
    expect(text(1)).toContain('$4.00, 20% under its usual $5.00'); // the alert, in words
    expect(container.querySelector('#cheap-1')?.classList.contains('alerting')).toBe(true);
    expect(text(1)).toContain('$4.40↑8% 2 wk');
    expect(text(2)).toContain('5% over usual');
    expect(text(3)).toContain('at usual');
    expect(text(4)).toContain('$6.00');

    await act(() => fireEvent.click(button(container, 'Set 1')));
    await waitFor(() => expect(container.querySelector('.detail svg')).not.toBeNull());
    expect(container.querySelector('.detail')?.textContent).toContain('usual $5.00');
    await act(() => fireEvent.click(button(container, 'Set 1'))); // closes
    expect(container.querySelector('.detail')).toBeNull();
  });

  it('watches a pack and says what will alert; snaps back if saving fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = mount([row(1)]);
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    const box = container.querySelector('tbody input[type=checkbox]') as HTMLInputElement;
    await act(() => fireEvent.click(box));
    await waitFor(() => expect(show).toHaveBeenCalledWith(expect.stringContaining("it's 15%+ under its usual price"), 'success'));
    expect(box.checked).toBe(true);
    await act(() => fireEvent.click(box));
    await waitFor(() => expect(box.checked).toBe(false));
  });

  it('keeps the box unticked when watching fails', async () => {
    const { container } = mount([row(1)], { canEdit: true }, false);
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    const box = container.querySelector('tbody input[type=checkbox]') as HTMLInputElement;
    await act(() => fireEvent.click(box));
    await waitFor(() => expect(box.checked).toBe(false));
  });

  it('filters by release date, or shows every set', async () => {
    const { container, fetchMock } = mount([row(1)], { canEdit: false, before: '2010-01-01' });
    await waitFor(() => expect(names(container)).toEqual(['Set 1']));
    expect(container.querySelector('tbody input')).toBeNull(); // viewers can't watch
    expect(fetchMock).toHaveBeenLastCalledWith('/api/cheapest/trends?before=2010-01-01', undefined);
    await act(() => fireEvent.submit(container.querySelector('form')!));
    expect(fetchMock).toHaveBeenLastCalledWith('/api/cheapest/trends?before=2010-01-01', undefined);
    await act(() => fireEvent.click(button(container, 'All sets')));
    await waitFor(() => expect(fetchMock).toHaveBeenLastCalledWith('/api/cheapest/trends', undefined));
    expect(button(container, 'All sets')).toBeUndefined();
  });

  it('says so when there is nothing to show or the API is down', async () => {
    const empty = mount([]);
    await waitFor(() => expect(empty.container.textContent).toContain('No packs found'));
    empty.unmount();
    const failed = mount(null);
    await waitFor(() => expect(failed.container.textContent).toContain("aren't available right now"));
  });
});

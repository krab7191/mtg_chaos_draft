import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import StoreDeals from './StoreDeals.svelte';
import { toast } from '../../lib/toast.svelte';
import type { OffersResponse, StoreOffer } from '../../lib/deals';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

const offer = (id: number, over: Partial<StoreOffer> = {}): StoreOffer => ({
  store: 'Collector Store', url: `https://x/${id}`, mtgstocksId: id, title: 't', price: 40, available: true,
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

const names = (c: HTMLElement) => Array.from(c.querySelectorAll('tbody td.name')).map(td => td.firstChild?.textContent?.trim());
const button = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('button')).find(b => b.textContent?.trim() === label)!;

describe('StoreDeals', () => {
  const resp: OffersResponse = {
    offers: [
      offer(1, { contents: 31 }),
      offer(2, { isDeal: false, discount: -0.1, price: 61.6, preorder: true, contents: 12, premium: true }),
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
    expect(container.querySelector('tbody a')?.getAttribute('href')).toBe('https://x/1');
    expect(Array.from(container.querySelectorAll('.elsewhere a')).map(a => a.textContent)).toContain('Card Kingdom');

    await act(() => fireEvent.click(button(container, 'In stock')));
    expect(names(container)).toEqual(['Set 1', 'Set 2']);
    expect(container.querySelector('tbody')?.textContent).toContain('10% over');
    expect(container.querySelector('tbody')?.textContent).toContain('In stock · Preorder');
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
    expect(container.textContent).toContain('No deals right now');
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

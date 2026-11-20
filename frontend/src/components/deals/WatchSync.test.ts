import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import StoreDeals from './StoreDeals.svelte';
import CheapestPacks from './CheapestPacks.svelte';
import { watched } from '../../lib/watches.svelte';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); watched.clear(); });

// Cheap Packs shows the same pack in two sections: one watch box ticks both.
describe('watch boxes on one page', () => {
  it('stay in sync between store listings and cheapest packs', async () => {
    const offer = { store: 'Card Kingdom', url: 'https://ck/1', mtgstocksId: 1, title: 'Set 1 Booster Pack', price: 4, available: true,
      seenAt: '', setName: 'Set 1', productType: 'Booster Pack', releasedAt: '2007-01-01', marketPrice: 5, discount: 0.2, isDeal: true };
    const row = { mtgstocksId: 1, name: 'Booster Pack', setName: 'Set 1', setCode: 'S', releasedAt: '2007-01-01',
      productType: 'Booster Pack', marketPrice: 5, trend: null, watch: null, alerts: [] };
    vi.stubGlobal('fetch', vi.fn(async (url: string): Promise<unknown> => {
      if (url === '/api/store-offers') return { ok: true, status: 200, json: async (): Promise<unknown> => ({ offers: [offer], stores: [], watched: [], dealDiscount: 0.15 }) };
      if (url.startsWith('/api/cheapest/trends')) return { ok: true, status: 200, json: async (): Promise<unknown> => ({ products: [row], dipBelowUsual: 0.15, muteAtStock: 20, notifications: false }) };
      return { ok: true, status: 204, json: async (): Promise<unknown> => ({}), text: async (): Promise<string> => '' };
    }));
    const store = render(StoreDeals, { props: { canEdit: true } });
    const cheapest = render(CheapestPacks, { props: { canEdit: true } });
    const boxIn = (c: HTMLElement) => c.querySelector('tbody input[type=checkbox]') as HTMLInputElement;
    await waitFor(() => expect(boxIn(store.container)).not.toBeNull());
    await waitFor(() => expect(boxIn(cheapest.container)).not.toBeNull());
    await act(() => fireEvent.click(boxIn(store.container)));
    await waitFor(() => expect(boxIn(cheapest.container).checked).toBe(true));
    await act(() => fireEvent.click(boxIn(cheapest.container)));
    await waitFor(() => expect(boxIn(store.container).checked).toBe(false));
  });
});

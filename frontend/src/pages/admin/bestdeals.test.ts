// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import BestDealsPage from './bestdeals.astro';

vi.mock('../../components/deals/BestDeals.svelte', () => import('../../tests/IslandStub.svelte'));

beforeEach(() => vi.unstubAllGlobals());

const me = (role: string) => vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ role, name: 'A', email: 'a@b.com' }) });

describe('BestDealsPage', () => {
  it('shows the page to admins and viewers', async () => {
    for (const role of ['admin', 'viewer']) {
      vi.stubGlobal('fetch', me(role));
      const res = await renderPage(BestDealsPage, '/admin/bestdeals');
      expect(res.status).toBe(200);
      expect(await res.text()).toContain('Best Deals');
    }
  });

  it('sends others away', async () => {
    vi.stubGlobal('fetch', me('user'));
    expect((await renderPage(BestDealsPage, '/admin/bestdeals')).headers.get('location')).toContain('/select');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce({ ok: false }));
    expect((await renderPage(BestDealsPage, '/admin/bestdeals')).headers.get('location')).toContain('/login');
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    expect((await renderPage(BestDealsPage, '/admin/bestdeals')).headers.get('location')).toContain('/login');
  });
});

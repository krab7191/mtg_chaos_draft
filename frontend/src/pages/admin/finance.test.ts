// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import FinancePage from './finance.astro';

// Islands have their own tests; stubs show what the page passes them.
vi.mock('../../components/finance/Finance.svelte', () => import('../../tests/IslandStub.svelte'));

beforeEach(() => vi.unstubAllGlobals());

const json = (body: unknown) => Promise.resolve({ ok: true, json: async () => body });

/** The props the page handed to the Finance island (rendered by the stub). */
function islandProps(html: string) {
  const raw = html.match(/<pre class="island-stub"[^>]*>([\s\S]*?)<\/pre>/)![1];
  return JSON.parse(raw.replace(/&quot;/g, '"').replace(/&amp;/g, '&').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>'));
}

describe('admin/finance page', () => {
  it('redirects to /login when fetch throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /login when not authenticated', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /select for non-admin role', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => json({ role: 'user', name: 'B', email: 'b@b.com' })));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.headers.get('location')).toContain('/select');
  });

  it('renders purchases for an admin', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (url.endsWith('/api/auth/me')) return json({ role: 'admin', name: 'A', email: 'a@b.com' });
      if (url.endsWith('/api/disposals')) return json([{
        id: 5, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', kind: 'draft', draftId: 3,
        quantity: 1, proceeds: 6, marketValue: 6, fees: 0, disposedOn: '2026-11-10',
      }]);
      if (url.endsWith('/api/purchases')) return json([{
        id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 2,
        totalPrice: 8, fees: 0, unitPrice: 4, currentPrice: 5, purchasedOn: '2026-11-01',
      }]);
      return json([{ id: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 1 }]);
    }));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Finance');
    expect(html).toContain('main--wide');
    const props = islandProps(html);
    expect(props.canEdit).toBe(true);
    expect(props.purchases[0].purchasedOn).toBe('2026-11-01');
    expect(props.disposals[0].kind).toBe('draft');
    expect(props.packs[0].setName).toBe('Bloomburrow');
  });

  it('still renders, read-only, for a viewer when the data cannot load', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) =>
      url.endsWith('/api/auth/me')
        ? json({ role: 'viewer', name: 'V', email: 'v@b.com' })
        : Promise.reject(new Error('offline'))));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.status).toBe(200);
    const html = await res.text();
    const props = islandProps(html);
    expect(props.canEdit).toBe(false);
    expect(props).toMatchObject({ purchases: [], disposals: [], packs: [] });
  });
});

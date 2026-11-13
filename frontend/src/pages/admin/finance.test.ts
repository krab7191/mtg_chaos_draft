// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import FinancePage from './finance.astro';

beforeEach(() => vi.unstubAllGlobals());

const json = (body: unknown) => Promise.resolve({ ok: true, json: async () => body });

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
      if (url.endsWith('/api/purchases')) return json([{
        id: 1, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 2,
        totalPrice: 8, fees: 0, unitPrice: 4, currentPrice: 5, purchasedOn: '2026-11-01',
      }]);
      return json([{ id: 1, setName: 'Bloomburrow', productType: 'Play Booster' }]);
    }));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Finance');
    expect(html).toContain('2026-11-01');
    expect(html).toContain('+ Log a purchase');
  });

  it('still renders, read-only, for a viewer when the data cannot load', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) =>
      url.endsWith('/api/auth/me')
        ? json({ role: 'viewer', name: 'V', email: 'v@b.com' })
        : Promise.reject(new Error('offline'))));
    const res = await renderPage(FinancePage, '/admin/finance');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('No purchases logged yet');
    expect(html).not.toContain('+ Log a purchase');
  });
});

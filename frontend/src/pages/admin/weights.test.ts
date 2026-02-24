// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import WeightsPage from './weights.astro';

beforeEach(() => vi.unstubAllGlobals());

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

describe('admin/weights page', () => {
  it('redirects to /login when not authenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(WeightsPage, '/admin/weights');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /select for non-admin role', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) },
    ]);
    const res = await renderPage(WeightsPage, '/admin/weights');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });

  it('renders empty message when no packs', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) },
      { ok: true, json: async () => [] },
      { ok: true, json: async () => ({ packWeights: {} }) },
    ]);
    const res = await renderPage(WeightsPage, '/admin/weights');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('add some first');
  });

  it('renders Weights heading with packs', async () => {
    const packs = [
      { id: 1, setName: 'Alpha', productType: 'Draft Booster', marketPrice: 10, quantity: 2 },
    ];
    mockFetch([
      { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) },
      { ok: true, json: async () => packs },
      { ok: true, json: async () => ({ packWeights: {}, priceFloor: 0, priceCap: 0, quantityCap: 0 }) },
    ]);
    const res = await renderPage(WeightsPage, '/admin/weights');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Weights');
  });
});

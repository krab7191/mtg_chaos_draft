// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import CheapPacksPage from './cheappacks.astro';

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

beforeEach(() => vi.unstubAllGlobals());

const packRow = { name: 'Test Pack', setName: 'TST', productType: 'Draft Booster', marketPrice: 4.99 };

describe('CheapPacksPage', () => {
  it('redirects to /login when unauthenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /select for non-admin role', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) },
    ]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });

  it('renders 200 for admin', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) },
      { ok: true, json: async () => [packRow] },
    ]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Test Pack');
    expect(html).toContain('4.99');
  });

  it('renders 200 for viewer role', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'viewer', name: 'View', email: 'v@b.com' }) },
      { ok: true, json: async () => [packRow] },
    ]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Test Pack');
  });

  it('shows empty state when no packs', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) },
      { ok: true, json: async () => [] },
    ]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('class="empty"');
  });

  it('defaults to sets released before 2020; clearing shows every set', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [packRow] });
    vi.stubGlobal('fetch', fetchMock);
    const html = await (await renderPage(CheapPacksPage, '/admin/cheappacks')).text();
    expect(fetchMock.mock.calls[1][0]).toContain('/api/cheapest?before=2020-01-01');
    expect(html).toContain('value="2020-01-01"');
    expect(html).toContain('href="/admin/cheappacks?before="');

    fetchMock.mockReset()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [packRow] });
    const cleared = await (await renderPage(CheapPacksPage, '/admin/cheappacks?before=')).text();
    expect(fetchMock.mock.calls[1][0]).toMatch(/\/api\/cheapest$/);
    expect(cleared).not.toContain('>Clear<');
  });
});

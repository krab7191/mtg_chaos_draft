// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import CheapPacksPage from './cheappacks.astro';

// The islands have their own tests; stubbing them keeps the page test (and the
// coverage of server-rendered island templates) about the page.
vi.mock('../../components/deals/StoreDeals.svelte', () => import('../../tests/IslandStub.svelte'));
vi.mock('../../components/deals/CheapestPacks.svelte', () => import('../../tests/IslandStub.svelte'));

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

beforeEach(() => vi.unstubAllGlobals());

const admin = { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) };

describe('CheapPacksPage', () => {
  it('redirects to /login when unauthenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /login when the API is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /select for non-admin role', async () => {
    mockFetch([{ ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) }]);
    const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });

  it('renders the store listings and cheapest packs for admins and viewers', async () => {
    for (const role of ['admin', 'viewer']) {
      mockFetch([{ ok: true, json: async () => ({ role, name: 'A', email: 'a@b.com' }) }]);
      const res = await renderPage(CheapPacksPage, '/admin/cheappacks');
      expect(res.status).toBe(200);
      const html = await res.text();
      expect(html).toContain('Cheap Packs');
      expect(html).toContain('store deals on single packs');
    }
  });

  it('defaults to sets released before 2020; ?before= shows every set', async () => {
    mockFetch([admin]);
    const html = await (await renderPage(CheapPacksPage, '/admin/cheappacks')).text();
    expect(html).toContain('2020-01-01'); // passed to the cheapest-packs island
    mockFetch([admin]);
    const all = await (await renderPage(CheapPacksPage, '/admin/cheappacks?before=')).text();
    expect(all).not.toContain('2020-01-01');
  });
});

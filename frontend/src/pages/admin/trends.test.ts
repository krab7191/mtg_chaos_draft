// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import TrendsPage from './trends.astro';

// The page test checks access and layout; the Trends component has its own
// tests. A stub keeps its server-rendered copy out of the coverage report.
vi.mock('../../components/trends/Trends.svelte', () => import('../../tests/IslandStub.svelte'));

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(responses[i++] ?? { ok: false })));
}

beforeEach(() => vi.unstubAllGlobals());

describe('Price Trends page', () => {
  it('redirects to /login when the API is down or the user is signed out', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    expect((await renderPage(TrendsPage, '/admin/trends')).headers.get('location')).toContain('/login');
    mockFetch([{ ok: false }]);
    expect((await renderPage(TrendsPage, '/admin/trends')).headers.get('location')).toContain('/login');
  });

  it('sends players to /select', async () => {
    mockFetch([{ ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) }]);
    expect((await renderPage(TrendsPage, '/admin/trends')).headers.get('location')).toContain('/select');
  });

  it('renders for admins and viewers, using the wide layout', async () => {
    for (const role of ['admin', 'viewer']) {
      mockFetch([{ ok: true, json: async () => ({ role, name: 'A', email: 'a@b.com' }) }]);
      const res = await renderPage(TrendsPage, '/admin/trends');
      expect(res.status).toBe(200);
      const html = await res.text();
      expect(html).toContain('Price Trends');
      expect(html).toContain('main--wide');
    }
  });
});

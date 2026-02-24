// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../tests/astro';
import SelectPage from './select.astro';

beforeEach(() => vi.unstubAllGlobals());

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

describe('select page', () => {
  it('redirects to /login when fetch throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const res = await renderPage(SelectPage, '/select');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /login when not authenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(SelectPage, '/select');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('renders 200 for authenticated user', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) },
      { ok: true, json: async () => [] },
      { ok: true, json: async () => ({ packWeights: {} }) },
    ]);
    const res = await renderPage(SelectPage, '/select');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Pick a Pack');
  });

  it('renders 200 even when collection fetch fails', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) },
      { ok: false },
      { ok: true, json: async () => ({ packWeights: {} }) },
    ]);
    const res = await renderPage(SelectPage, '/select');
    expect(res.status).toBe(200);
  });
});

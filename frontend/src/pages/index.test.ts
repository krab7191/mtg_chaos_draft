// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../tests/astro';
import IndexPage from './index.astro';

beforeEach(() => vi.unstubAllGlobals());

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

describe('index page', () => {
  it('redirects to /login when fetch throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const res = await renderPage(IndexPage, '/');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /login when not authenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(IndexPage, '/');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects admin to /admin/collection', async () => {
    mockFetch([{ ok: true, json: async () => ({ role: 'admin' }) }]);
    const res = await renderPage(IndexPage, '/');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/admin/collection');
  });

  it('redirects viewer to /admin/collection', async () => {
    mockFetch([{ ok: true, json: async () => ({ role: 'viewer' }) }]);
    const res = await renderPage(IndexPage, '/');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/admin/collection');
  });

  it('redirects regular user to /select', async () => {
    mockFetch([{ ok: true, json: async () => ({ role: 'user' }) }]);
    const res = await renderPage(IndexPage, '/');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });
});

// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../../tests/astro';
import DraftsPage from './drafts.astro';

beforeEach(() => vi.unstubAllGlobals());

function mockFetch(responses: Array<{ ok: boolean; json?: () => Promise<unknown> }>) {
  let i = 0;
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
    Promise.resolve(responses[i++] ?? { ok: false }),
  ));
}

describe('admin/drafts page', () => {
  it('redirects to /login when not authenticated', async () => {
    mockFetch([{ ok: false }]);
    const res = await renderPage(DraftsPage, '/admin/drafts');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/login');
  });

  it('redirects to /select for non-admin role', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'user', name: 'Bob', email: 'b@b.com' }) },
    ]);
    const res = await renderPage(DraftsPage, '/admin/drafts');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });

  it('renders 200 with Drafts heading for admin', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'admin', name: 'Alice', email: 'a@b.com' }) },
      { ok: true, json: async () => [] },
    ]);
    const res = await renderPage(DraftsPage, '/admin/drafts');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Drafts');
  });

  it('renders 200 for viewer role', async () => {
    mockFetch([
      { ok: true, json: async () => ({ role: 'viewer', name: 'View', email: 'v@b.com' }) },
      { ok: true, json: async () => [] },
    ]);
    const res = await renderPage(DraftsPage, '/admin/drafts');
    expect(res.status).toBe(200);
  });
});

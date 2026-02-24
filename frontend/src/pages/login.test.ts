// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderPage } from '../tests/astro';
import LoginPage from './login.astro';

beforeEach(() => vi.unstubAllGlobals());

describe('login page', () => {
  it('renders sign-in page when API is unavailable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const res = await renderPage(LoginPage, '/login');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Sign in with Google');
  });

  it('renders sign-in page when not authenticated', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
    const res = await renderPage(LoginPage, '/login');
    expect(res.status).toBe(200);
    const html = await res.text();
    expect(html).toContain('Sign in with Google');
  });

  it('redirects admin to /admin/collection when already logged in', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ role: 'admin' }),
    }));
    const res = await renderPage(LoginPage, '/login');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/admin/collection');
  });

  it('redirects user to /select when already logged in', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ role: 'user' }),
    }));
    const res = await renderPage(LoginPage, '/login');
    expect(res.status).toBe(302);
    expect(res.headers.get('location')).toContain('/select');
  });
});

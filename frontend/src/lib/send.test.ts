import { describe, it, expect, vi, afterEach } from 'vitest';
import { send } from './send';
import { toast } from './toast.svelte';

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

const spyShow = () => vi.spyOn(toast, 'show');

describe('send', () => {
  it('sends JSON and returns the parsed response', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ id: 1 }) });
    vi.stubGlobal('fetch', fetchMock);
    expect(await send('/api/x', 'PUT', { a: 1 }, 'Failed')).toEqual({ id: 1 });
    expect(fetchMock).toHaveBeenCalledWith('/api/x', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: '{"a":1}',
    });
  });

  it('returns true for an empty response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204 }));
    expect(await send('/api/x', 'DELETE', null, 'Failed')).toBe(true);
  });

  it('shows the server message on an error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 409, text: async () => 'not enough packs in stock\n' }));
    const show = spyShow();
    expect(await send('/api/x', 'POST', {}, 'Failed to record sale')).toBeNull();
    expect(show).toHaveBeenCalledWith('Failed to record sale: not enough packs in stock', 'error');
  });

  it('describes the status when the server sends nothing useful, and handles network errors', async () => {
    const show = spyShow();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: async () => { throw new Error('x'); } }));
    expect(await send('/api/x', 'POST', {}, 'Failed')).toBeNull();
    expect(show).toHaveBeenLastCalledWith('Failed: the server hit an error (500)', 'error');
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    expect(await send('/api/x', 'POST', {}, 'Offline')).toBeNull();
    expect(show).toHaveBeenLastCalledWith("Offline: can't reach the server — check your connection", 'error');
  });

  it('treats a successful response without a JSON body as success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => { throw new SyntaxError('x'); } }));
    expect(await send('/api/x', 'PUT', {}, 'Failed')).toBe(true);
  });
});

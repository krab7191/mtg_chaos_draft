import { describe, it, expect, vi, afterEach } from 'vitest';
import { send } from './send';
import { toast } from './toast.svelte';

afterEach(() => { vi.unstubAllGlobals(); toast.dismiss(); });

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
    expect(await send('/api/x', 'POST', {}, 'Failed to record sale')).toBeNull();
    expect(toast.msg).toBe('Failed to record sale: not enough packs in stock');
    expect(toast.type).toBe('error');
  });

  it('falls back to the status code, and handles network errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: async () => { throw new Error('x'); } }));
    expect(await send('/api/x', 'POST', {}, 'Failed')).toBeNull();
    expect(toast.msg).toBe('Failed: 500');
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    expect(await send('/api/x', 'POST', {}, 'Offline')).toBeNull();
    expect(toast.msg).toBe('Offline');
  });
});

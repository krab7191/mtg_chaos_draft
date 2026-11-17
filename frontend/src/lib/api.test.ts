import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { describeStatus, errorReason, getJSON, NETWORK_ERROR } from './api';
import { toast } from './toast.svelte';

let show: ReturnType<typeof vi.spyOn>;
beforeEach(() => { show = vi.spyOn(toast, 'show'); });
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

const res = (status: number, body?: string) =>
  ({ ok: status < 400, status, text: async () => body ?? '', json: async () => JSON.parse(body ?? 'null') }) as Response;

describe('describeStatus', () => {
  it.each([
    [400, 'rejected'], [401, 'sign in'], [403, 'permission'], [404, "couldn't be found"],
    [409, 'conflicts'], [429, 'too many'], [502, 'unreachable'], [503, 'unreachable'],
    [504, 'unreachable'], [500, 'error (500)'], [418, 'failed (418)'],
  ])('%i mentions "%s"', (status, text) => {
    expect(describeStatus(status)).toContain(text);
  });
});

describe('errorReason', () => {
  it("uses the server's short plain-text message", async () => {
    expect(await errorReason(res(409, 'not enough stock\n'))).toBe('not enough stock');
  });

  it('ignores HTML pages, overlong bodies and empty ones', async () => {
    expect(await errorReason(res(502, '<html>Bad Gateway</html>'))).toContain('unreachable');
    expect(await errorReason(res(500, 'x'.repeat(201)))).toContain('(500)');
    expect(await errorReason(res(500, '   '))).toContain('(500)');
    expect(await errorReason({ status: 404 } as Response)).toContain("couldn't be found");
  });
});

describe('getJSON', () => {
  it('returns parsed JSON on success without toasting', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(res(200, '[1,2]')));
    expect(await getJSON('/api/x', 'Failed')).toEqual([1, 2]);
    expect(show).not.toHaveBeenCalled();
  });

  it('toasts and returns null on an error status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(res(502, "couldn't reach MTGStocks")));
    expect(await getJSON('/api/x', 'Search failed')).toBeNull();
    expect(show).toHaveBeenCalledWith("Search failed: couldn't reach MTGStocks", 'error');
  });

  it('toasts on network failure but stays quiet when aborted', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
    expect(await getJSON('/api/x', 'Search failed')).toBeNull();
    expect(show).toHaveBeenCalledWith(`Search failed: ${NETWORK_ERROR}`, 'error');

    show.mockClear();
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new DOMException('aborted', 'AbortError')));
    expect(await getJSON('/api/x', 'Search failed')).toBeNull();
    expect(show).not.toHaveBeenCalled();
  });
});

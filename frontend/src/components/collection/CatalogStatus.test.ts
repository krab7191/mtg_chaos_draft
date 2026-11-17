import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act, waitFor } from '../../tests/svelte';
import CatalogStatus from './CatalogStatus.svelte';
import { toast } from '../../lib/toast.svelte';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

const ago = (ms: number) => new Date(Date.now() - ms).toISOString();
const json = (body: unknown, status = 200) =>
  ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) });

async function mount(status: unknown, isAdmin = true) {
  const fetchMock = vi.fn().mockResolvedValue(json(status));
  vi.stubGlobal('fetch', fetchMock);
  const r = render(CatalogStatus, { props: { isAdmin } });
  await waitFor(() => expect(r.container.querySelector('.catalog')).not.toBeNull());
  return { ...r, fetchMock, text: () => r.container.querySelector('.catalog__text')?.textContent ?? '' };
}

describe('CatalogStatus', () => {
  it('shows a fresh live catalog calmly', async () => {
    const { text, container } = await mount({ available: true, fetchedAt: ago(5 * 60_000), source: 'live', products: 900 });
    expect(text()).toContain('Prices live from MTGStocks, updated 5 min ago');
    expect(container.querySelector('.catalog--warn')).toBeNull();
  });

  it('warns when the live catalog is stale', async () => {
    const { text, container } = await mount({ available: true, fetchedAt: ago(5 * 3_600_000), source: 'live', products: 900 });
    expect(text()).toContain('MTGStocks unreachable — using prices from 5 h ago');
    expect(container.querySelector('.catalog--warn')).not.toBeNull();
  });

  it('flags an uploaded snapshot with its age', async () => {
    const { text } = await mount({ available: true, fetchedAt: ago(3 * 86_400_000), source: 'upload', products: 900 });
    expect(text()).toContain('uploaded snapshot, 3 days ago (900 products)');
  });

  it('tells viewers an admin must upload, and hides the button', async () => {
    const { text, container } = await mount({ available: false, fetchedAt: null, source: '', products: 0 }, false);
    expect(text()).toContain('until an admin uploads one');
    expect(container.querySelector('.catalog__upload')).toBeNull();
  });

  it('renders nothing when the status request fails', async () => {
    const show = vi.spyOn(toast, 'show');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: async () => '' }));
    const { container } = render(CatalogStatus, { props: { isAdmin: true } });
    await waitFor(() => expect(show).toHaveBeenCalled());
    expect(container.querySelector('.catalog')).toBeNull();
  });

  it('uploads a snapshot and shows the new status', async () => {
    const { container, fetchMock, text } = await mount({ available: false, fetchedAt: null, source: '', products: 0 });
    const success = vi.spyOn(toast, 'success');
    fetchMock.mockResolvedValueOnce(json({ available: true, fetchedAt: ago(0), source: 'upload', products: 42 }));

    const input = container.querySelector('input[type=file]') as HTMLInputElement;
    const file = new File(['[]'], 'sealed.json', { type: 'application/json' });
    Object.defineProperty(input, 'files', { value: [file], configurable: true });
    await act(() => fireEvent.change(input));

    await waitFor(() => expect(text()).toContain('(42 products)'));
    expect(fetchMock).toHaveBeenLastCalledWith('/api/catalog', expect.objectContaining({ method: 'PUT', body: file }));
    expect(success).toHaveBeenCalledWith(expect.stringContaining('42 products'));
  });

  it('reports a rejected upload and a network failure', async () => {
    const { container, fetchMock } = await mount({ available: false, fetchedAt: null, source: '', products: 0 });
    const show = vi.spyOn(toast, 'show');
    const input = container.querySelector('input[type=file]') as HTMLInputElement;
    Object.defineProperty(input, 'files', { value: [new File(['x'], 'x.json')], configurable: true });

    fetchMock.mockResolvedValueOnce({ ok: false, status: 400, text: async () => 'not an MTGStocks sealed catalog' });
    await act(() => fireEvent.change(input));
    await waitFor(() => expect(show).toHaveBeenCalledWith('Catalog upload failed: not an MTGStocks sealed catalog', 'error'));

    fetchMock.mockRejectedValueOnce(new TypeError('offline'));
    await act(() => fireEvent.change(input));
    await waitFor(() => expect(show).toHaveBeenLastCalledWith(expect.stringContaining("can't reach the server"), 'error'));
  });

  it('ignores an empty file selection', async () => {
    const { container, fetchMock } = await mount({ available: false, fetchedAt: null, source: '', products: 0 });
    const input = container.querySelector('input[type=file]') as HTMLInputElement;
    Object.defineProperty(input, 'files', { value: [], configurable: true });
    await act(() => fireEvent.change(input));
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

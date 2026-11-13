import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import Finance from './Finance.svelte';
import { toast } from '../../lib/toast.svelte';
import { localToday } from '../../lib/purchases';

const packs = [
  { id: 1, setName: 'Bloomburrow', productType: 'Play Booster' },
  { id: 2, setName: 'Aetherdrift', productType: 'Play Booster' },
];
const display = {
  id: 10, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 30,
  totalPrice: 132.18, fees: 10.68, unitPrice: 4.406, currentPrice: 5, purchasedOn: '2026-11-01',
};
const single = {
  id: 9, packId: 2, setName: 'Aetherdrift', productType: 'Play Booster', quantity: 1,
  totalPrice: 5, fees: 0, unitPrice: 5, currentPrice: 4, purchasedOn: '2026-10-01',
};
const purchases = [display, single];
const flush = () => new Promise(r => setTimeout(r, 0));

afterEach(() => { vi.unstubAllGlobals(); toast.dismiss(); });

function openForm(container: HTMLElement) {
  return act(() => fireEvent.click(container.querySelector('.btn-link')!));
}

describe('Finance overview', () => {
  it('summarises spend, value, and gain with and without tax & shipping', () => {
    const { container } = render(Finance, { props: { purchases, packs } });
    const cards = Array.from(container.querySelectorAll('.card')).map(c => c.textContent!.replace(/\s+/g, ' '));
    expect(cards[0]).toContain('$137.18');
    expect(cards[0]).toContain('31 packs');
    expect(cards[0]).toContain('incl. $10.68 tax & shipping');
    expect(cards[1]).toContain('$154.00');           // 30×5 + 1×4
    expect(cards[2]).toContain('+$27.50');           // 154 − (121.50 + 5)
    expect(cards[2]).toContain('+21.7%');
    expect(cards[2]).toContain('+$16.82 after tax & shipping');
  });

  it('shows a loss in red', () => {
    const { container } = render(Finance, { props: { purchases: [single], packs } });
    const gain = container.querySelectorAll('.card__value')[2];
    expect(gain.textContent).toContain('−$1.00');
    expect(gain.classList.contains('tone--bad')).toBe(true);
  });

  it('lists holdings per pack', () => {
    const { container } = render(Finance, { props: { purchases, packs } });
    const rows = container.querySelectorAll('.holdings tbody tr');
    expect(rows).toHaveLength(2);
    expect(rows[0].textContent).toContain('Bloomburrow');
    expect(rows[0].textContent).toContain('$4.05'); // 121.50 / 30
    expect(rows[0].textContent).toContain('+23.5%');
  });

  it('explains how to start when nothing is logged', () => {
    const { container } = render(Finance, { props: { purchases: [], packs } });
    expect(container.querySelector('.empty')).not.toBeNull();
    expect(container.querySelector('.holdings')).toBeNull();
    expect(container.querySelector('.btn-link')).toBeNull(); // read-only without canEdit
  });
});

describe('Finance purchase log', () => {
  it('shows each purchase against today\'s market, judged without tax & shipping', () => {
    const { container } = render(Finance, { props: { purchases, packs } });
    const items = container.querySelectorAll('.log__item');
    expect(items[0].textContent).toContain('$132.18 for 30 packs');
    expect(items[0].textContent).toContain('($4.05 each)');
    expect(items[0].textContent).toContain('incl. $10.68 tax & shipping');
    expect(items[0].textContent).toContain('19% under');
    expect(items[1].textContent).toContain('25% over');
    expect(container.querySelector('.btn-row')).toBeNull();
  });

  it('logs a purchase from a receipt with a total and fees', async () => {
    const created = { ...single, id: 11, purchasedOn: localToday() };
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => created });
    vi.stubGlobal('fetch', fetchMock);
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await openForm(container);
    expect(container.querySelector('option:not([disabled])')?.textContent).toContain('Aetherdrift');

    const save = container.querySelector('.form .btn-save') as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    await act(() => fireEvent.change(container.querySelector('select')!, { target: { value: '2' } }));
    const nums = container.querySelectorAll<HTMLInputElement>('.form input[type=number]');
    await act(() => fireEvent.input(nums[0], { target: { value: '30' } }));
    await act(() => fireEvent.input(nums[1], { target: { value: '132.18' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '10.68' } }));
    expect(save.disabled).toBe(false);
    await act(() => fireEvent.submit(container.querySelector('form')!));
    await act(flush);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/purchases');
    expect(JSON.parse(init.body)).toEqual({
      packId: 2, quantity: 30, paidTotal: 132.18, fees: 10.68, purchasedOn: localToday(), addToStock: false,
    });
    expect(container.querySelectorAll('.log__item')).toHaveLength(3);
    // Ready for the next receipt line.
    expect((container.querySelector('select') as HTMLSelectElement).value).toBe('');
    expect(nums[1].value).toBe('');
  });

  it('marks packs with no logged price', async () => {
    const { container } = render(Finance, { props: { purchases: [display], packs, canEdit: true } });
    await openForm(container);
    const options = Array.from(container.querySelectorAll('option')).map(o => o.textContent);
    expect(options).toContain('Aetherdrift — Play Booster · no price yet');
    expect(container.querySelector('.form__hint')!.textContent).toContain('1 pack type with no price yet');
  });

  it('shows the server error when logging fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, text: async () => "fees can't be more than the total paid\n" }));
    const { container } = render(Finance, { props: { purchases: [], packs, canEdit: true } });
    await openForm(container);
    await act(() => fireEvent.change(container.querySelector('select')!, { target: { value: '1' } }));
    const nums = container.querySelectorAll<HTMLInputElement>('.form input[type=number]');
    await act(() => fireEvent.input(nums[1], { target: { value: '4' } }));
    await act(() => fireEvent.submit(container.querySelector('form')!));
    await act(flush);
    expect(toast.msg).toContain("fees can't be more than the total paid");
  });

  it('reports a network failure when logging', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const { container } = render(Finance, { props: { purchases: [], packs, canEdit: true } });
    await openForm(container);
    await act(() => fireEvent.change(container.querySelector('select')!, { target: { value: '1' } }));
    const nums = container.querySelectorAll<HTMLInputElement>('.form input[type=number]');
    await act(() => fireEvent.input(nums[1], { target: { value: '4' } }));
    await act(() => fireEvent.submit(container.querySelector('form')!));
    await act(flush);
    expect(toast.msg).toBe('Failed to log purchase');
  });
});

describe('Finance editing', () => {
  function editButton(container: HTMLElement) {
    return Array.from(container.querySelectorAll('.btn-row')).find(b => b.textContent === 'Edit')!;
  }

  it('edits a purchase in place', async () => {
    const updated = { ...display, totalPrice: 135, fees: 12 };
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => updated });
    vi.stubGlobal('fetch', fetchMock);
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });

    await act(() => fireEvent.click(editButton(container)));
    const form = container.querySelector('.form--inline')!;
    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    expect(nums[0].value).toBe('30');
    expect(nums[1].value).toBe('132.18');
    expect(nums[2].value).toBe('10.68');
    await act(() => fireEvent.input(nums[1], { target: { value: '135' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '12' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/purchases/10');
    expect(init.method).toBe('PUT');
    expect(JSON.parse(init.body)).toEqual({ quantity: 30, paidTotal: 135, fees: 12, purchasedOn: '2026-11-01' });
    expect(container.querySelector('.form--inline')).toBeNull();
    expect(container.querySelector('.log__item')!.textContent).toContain('$135.00 for 30 packs');
  });

  it('can cancel an edit', async () => {
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await act(() => fireEvent.click(editButton(container)));
    const cancel = Array.from(container.querySelectorAll('.btn-row')).find(b => b.textContent === 'Cancel')!;
    await act(() => fireEvent.click(cancel));
    expect(container.querySelector('.form--inline')).toBeNull();
  });

  it('keeps the edit open when saving fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, text: async () => '' }));
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await act(() => fireEvent.click(editButton(container)));
    await act(() => fireEvent.submit(container.querySelector('.form--inline')!));
    await act(flush);
    expect(toast.msg).toBe('Failed to save: 400');
    expect(container.querySelector('.form--inline')).not.toBeNull();
  });

  it('reports a network failure when saving', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await act(() => fireEvent.click(editButton(container)));
    await act(() => fireEvent.submit(container.querySelector('.form--inline')!));
    await act(flush);
    expect(toast.msg).toBe('Failed to save purchase');
  });

  it('deletes a purchase after confirming', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal('fetch', fetchMock);
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await act(() => fireEvent.click(container.querySelector('.btn-row--danger')!));
    expect(toast.mode).toBe('confirm');
    await act(async () => { toast.onConfirm!(); await flush(); });
    expect(fetchMock).toHaveBeenCalledWith('/api/purchases/10', { method: 'DELETE' });
    expect(container.querySelectorAll('.log__item')).toHaveLength(1);
  });

  it('keeps the purchase when deleting fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, statusText: 'oops' }));
    const { container } = render(Finance, { props: { purchases, packs, canEdit: true } });
    await act(() => fireEvent.click(container.querySelector('.btn-row--danger')!));
    await act(async () => { toast.onConfirm!(); await flush(); });
    expect(container.querySelectorAll('.log__item')).toHaveLength(2);
    expect(toast.type).toBe('error');
  });
});

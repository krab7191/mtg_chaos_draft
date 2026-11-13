import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import Finance from './Finance.svelte';
import { toast } from '../../lib/toast.svelte';
import { localToday } from '../../lib/purchases';

const packs = [
  { id: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 12, marketPrice: 5 },
  { id: 2, setName: 'Aetherdrift', productType: 'Play Booster', quantity: 0, marketPrice: 4 },
];
const display = {
  id: 10, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', quantity: 30,
  totalPrice: 132.18, fees: 10.68, unitPrice: 4.406, currentPrice: 5, purchasedOn: '2026-10-01',
};
const draft = {
  id: 20, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', kind: 'draft' as const, draftId: 7,
  quantity: 18, proceeds: 60, marketValue: 90, fees: 0, disposedOn: '2026-11-12',
};
const sale = {
  id: 21, packId: 1, setName: 'Bloomburrow', productType: 'Play Booster', kind: 'sale' as const, draftId: null,
  quantity: 2, proceeds: 12, marketValue: null, fees: 1, disposedOn: '2026-11-13',
};
const flush = () => new Promise(r => setTimeout(r, 0));
const ok = (body: unknown, status = 200) => ({ ok: true, status, json: async () => body });

afterEach(() => { vi.unstubAllGlobals(); toast.dismiss(); });

function setup(props: Record<string, unknown> = {}) {
  return render(Finance, { props: { purchases: [display], disposals: [draft, sale], packs, canEdit: true, ...props } });
}
const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, ' ') ?? '';
function button(container: HTMLElement, label: string, scope: ParentNode = container) {
  return Array.from(scope.querySelectorAll('button')).find(b => b.textContent?.includes(label)) as HTMLButtonElement;
}
async function openPicker(scope: ParentNode) {
  const input = scope.querySelector<HTMLInputElement>('input[type=search]')!;
  await act(() => fireEvent.focus(input));
  return input;
}
const pickerItems = (scope: ParentNode) => Array.from(scope.querySelectorAll('.picker__item')).map(text);
async function pick(scope: ParentNode, name: string) {
  const input = await openPicker(scope);
  await act(() => fireEvent.input(input, { target: { value: name } }));
  const item = Array.from(scope.querySelectorAll('.picker__item')).find(b => b.textContent?.includes(name))!;
  await act(() => fireEvent.click(item));
}
const rowOf = (container: HTMLElement, tag: string) =>
  Array.from(container.querySelectorAll('.log__item')).find(li => li.querySelector('.tag')?.textContent === tag)!;

describe('Finance overview', () => {
  it('values stock and splits gains into realized and unrealized', () => {
    const { container } = setup();
    // 30 bought at $4.05 (121.50 without fees); 18 drafted, 2 sold → 10 logged left; 12 in stock.
    expect(text(container.querySelector('.invested'))).toContain('Invested $132.18 in 30 packs logged (incl. $10.68 tax & shipping)');
    const cards = Array.from(container.querySelectorAll('.card')).map(text);
    expect(cards[0]).toContain('$60.00');            // all 12 in stock × $5
    expect(cards[0]).toContain('12 packs in stock');
    expect(cards[0]).toContain('2 with no logged cost');
    expect(cards[1]).toContain('+$9.50');            // 10 × $5 − $40.50
    expect(cards[1]).toContain('+23.5%');
    expect(cards[1]).toContain('logged cost of $40.50');
    // Draft: 60 − 72.90 = −12.90. Sale: 11 − 8.10 = +2.90.
    expect(cards[2]).toContain('−$10.00');
    expect(cards[2]).toContain('$71.00 collected');
    expect(cards[3]).toContain('$0.00');
  });

  it('counts logged packs missing from stock as opened', () => {
    const { container } = setup({ disposals: [], packs: [{ ...packs[0], quantity: 8 }] });
    const cards = Array.from(container.querySelectorAll('.card')).map(text);
    expect(cards[3]).toContain('$89.10');            // 22 × $4.05
    expect(cards[3]).toContain('cost of 22 packs logged but no longer in stock');
    expect(cards[2]).toContain('+$0.00');            // opening isn't a loss
    expect(text(container.querySelector('.holdings tbody tr'))).toContain('30 bought · 22 opened');
  });

  it('shows holdings per pack from stock', () => {
    const { container } = setup();
    const row = container.querySelector('.holdings tbody tr')!;
    expect(text(row)).toContain('30 bought · 20 sold/drafted · 2 with no logged cost');
    expect(text(row)).toContain('12');
    expect(text(row)).toContain('$4.05');
    expect(text(row)).toContain('$60.00');
    expect(text(row)).toContain('−$10.00');
  });

  it('dims a pack that is no longer in stock', () => {
    const { container } = setup({ packs: [{ ...packs[0], quantity: 0 }] });
    expect(container.querySelector('.holdings tr.closed')).not.toBeNull();
  });

  it('explains how to start when there is nothing yet, read-only for viewers', () => {
    const { container } = setup({ purchases: [], disposals: [], canEdit: false });
    expect(container.querySelector('.empty')).not.toBeNull();
    expect(container.querySelector('.invested')).toBeNull();
    // Stock still shows, valued at market, with no cost.
    const row = container.querySelector('.holdings tbody tr')!;
    expect(text(row)).toContain('$60.00');
    expect(text(row).match(/—/g)?.length).toBeGreaterThanOrEqual(3);
    expect(button(container, 'Log a purchase')).toBeUndefined();
  });
});

describe('Finance activity', () => {
  it('lists buys, drafts and sales with their results', () => {
    const { container } = setup();
    expect(Array.from(container.querySelectorAll('.tag')).map(t => t.textContent)).toEqual(['Sale', 'Draft', 'Buy']);
    expect(text(rowOf(container, 'Sale'))).toContain('$12.00 for 2 packs ($6.00 each) · $1.00 fees');
    expect(text(rowOf(container, 'Sale'))).toContain('+$2.90 gain');
    expect(text(rowOf(container, 'Draft'))).toContain('18 packs opened');
    expect(text(rowOf(container, 'Draft'))).toContain('$60.00 collected · market $90.00');
    expect(text(rowOf(container, 'Draft'))).toContain('−$12.90 gain');
    expect(text(rowOf(container, 'Buy'))).toContain('$132.18 for 30 packs ($4.05 each) · incl. $10.68 tax & shipping');
    expect(text(rowOf(container, 'Buy'))).toContain('19% under');
  });

  it('says when a draft or sale has no logged cost', () => {
    const { container } = setup({ purchases: [] });
    expect(text(rowOf(container, 'Draft'))).toContain('no logged cost');
    expect(text(rowOf(container, 'Sale'))).toContain('no logged cost');
  });

  it('notes packs without a cost in a partly covered draft', () => {
    const { container } = setup({ purchases: [{ ...display, quantity: 10, totalPrice: 40, fees: 0 }], disposals: [draft] });
    expect(text(rowOf(container, 'Draft'))).toContain('8 packs with no logged cost');
  });

  it('can\'t edit a draft whose draft was deleted', () => {
    const { container } = setup({ disposals: [{ ...draft, draftId: null }] });
    expect(button(container, 'Edit', rowOf(container, 'Draft'))).toBeUndefined();
  });
});

describe('Finance purchases', () => {
  it('logs a purchase from a receipt and keeps the date for the next line', async () => {
    const created = { ...display, id: 11, packId: 2, setName: 'Aetherdrift', purchasedOn: localToday() };
    const fetchMock = vi.fn().mockResolvedValue(ok(created, 201));
    vi.stubGlobal('fetch', fetchMock);
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Log a purchase')));
    const form = container.querySelector('.form')!;
    await openPicker(form);
    expect(pickerItems(form)).toContain('Aetherdrift — Play Booster · no price yet');
    expect(text(form)).toContain('1 pack type with no price yet');

    await pick(form, 'Aetherdrift');
    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    await act(() => fireEvent.input(nums[0], { target: { value: '30' } }));
    await act(() => fireEvent.input(nums[1], { target: { value: '132.18' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '10.68' } }));
    await act(() => fireEvent.click(form.querySelector('.check input')!));
    await act(() => fireEvent.submit(form));
    await act(flush);

    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      packId: 2, quantity: 30, paidTotal: 132.18, fees: 10.68, purchasedOn: localToday(), addToStock: true,
    });
    expect(container.querySelectorAll('.tag--buy')).toHaveLength(2);
    expect(form.querySelector<HTMLInputElement>('input[type=search]')!.value).toBe('');
    // Added to stock, so it can now be sold.
    await act(() => fireEvent.click(button(container, 'Record a sale')));
    await openPicker(container.querySelector('.form')!);
    expect(pickerItems(container)).toContain('Aetherdrift — Play Booster (30 in stock)');
  });

  it('keeps the form when logging fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 400, text: async () => 'bad' }));
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Log a purchase')));
    const form = container.querySelector('.form')!;
    await pick(form, 'Bloomburrow');
    await act(() => fireEvent.input(form.querySelectorAll('input[type=number]')[1], { target: { value: '4' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);
    expect(toast.msg).toBe('Failed to log purchase: bad');
    expect(form.querySelector<HTMLInputElement>('input[type=search]')!.value).toBe('Bloomburrow — Play Booster');
  });

  it('edits a purchase in place', async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok({ ...display, totalPrice: 135, fees: 12 }));
    vi.stubGlobal('fetch', fetchMock);
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Edit', rowOf(container, 'Buy'))));
    const form = container.querySelector('.form--inline')!;
    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    expect([nums[0].value, nums[1].value, nums[2].value]).toEqual(['30', '132.18', '10.68']);
    await act(() => fireEvent.input(nums[1], { target: { value: '135' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '12' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/purchases/10');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ quantity: 30, paidTotal: 135, fees: 12, purchasedOn: '2026-10-01' });
    expect(container.querySelector('.form--inline')).toBeNull();
    expect(text(rowOf(container, 'Buy'))).toContain('$135.00 for 30 packs');
  });

  it('cancels an edit', async () => {
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Edit', rowOf(container, 'Buy'))));
    await act(() => fireEvent.click(button(container, 'Cancel')));
    expect(container.querySelector('.form--inline')).toBeNull();
  });

  it('deletes a purchase after confirming', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204 }));
    const { container } = setup();
    await act(() => fireEvent.click(rowOf(container, 'Buy').querySelector('.btn-row--danger')!));
    expect(toast.mode).toBe('confirm');
    await act(async () => { toast.onConfirm!(); await flush(); });
    expect(container.querySelector('.tag--buy')).toBeNull();
  });
});

describe('Finance purchases of packs not in the collection', () => {
  it('finds a set on MTGStocks and adds it with the purchase, without stock', async () => {
    const product = {
      mtgstocksId: 501, name: 'Homelands Booster', setName: 'Homelands', setCode: 'HML',
      productType: 'Booster Pack', marketPrice: 15,
    };
    const added = {
      id: 3, setName: 'Homelands', productType: 'Booster Pack', quantity: 0, marketPrice: 15, mtgstocksId: 501,
      purchase: {
        id: 30, packId: 3, setName: 'Homelands', productType: 'Booster Pack', quantity: 4,
        totalPrice: 55.92, fees: 5.92, unitPrice: 13.98, currentPrice: 15, purchasedOn: '2026-03-29',
      },
    };
    const fetchMock = vi.fn().mockImplementation((url: string) =>
      Promise.resolve(url.startsWith('/api/search')
        ? ok([product, { ...product, mtgstocksId: 999, setName: 'Bloomburrow', productType: 'Play Booster' }])
        : ok(added, 201)));
    // The Bloomburrow result is already in the collection (by MTGStocks ID), so it isn't offered twice.
    vi.stubGlobal('fetch', fetchMock);
    const { container } = setup({ packs: [{ ...packs[0], mtgstocksId: 999 }, packs[1]] });
    await act(() => fireEvent.click(button(container, 'Log a purchase')));
    const form = container.querySelector('.form')!;

    const input = await openPicker(form);
    await act(() => fireEvent.input(input, { target: { value: 'homelands' } }));
    expect(text(form.querySelector('.picker__list'))).toContain('Searching…');
    await act(() => new Promise(r => setTimeout(r, 450)));
    expect(fetchMock.mock.calls[0][0]).toBe('/api/search?q=homelands');
    expect(pickerItems(form)).toEqual(['Homelands — Booster Pack $15.00']);
    await act(() => fireEvent.click(form.querySelector('.picker__item')!));
    expect(text(form)).toContain("Not in your collection yet: it'll be added.");

    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    await act(() => fireEvent.input(nums[0], { target: { value: '4' } }));
    await act(() => fireEvent.input(nums[1], { target: { value: '55.92' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '5.92' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);

    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe('/api/collection');
    expect(JSON.parse(init.body)).toEqual({
      mtgstocksId: 501, name: 'Homelands Booster', setName: 'Homelands', setCode: 'HML',
      productType: 'Booster Pack', marketPrice: 15, weight: 1,
      quantity: 4, paidTotal: 55.92, fees: 5.92, purchasedOn: localToday(), addToStock: false,
    });
    expect(text(container.querySelector('.log'))).toContain('$55.92 for 4 packs');
    // Now it's in the collection, so the picker offers it from there.
    await openPicker(form);
    expect(pickerItems(form)).toContain('Homelands — Booster Pack');
  });

  it('says when nothing else matches, and ignores a failed search', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Log a purchase')));
    const form = container.querySelector('.form')!;
    const input = await openPicker(form);
    await act(() => fireEvent.input(input, { target: { value: 'zzz' } }));
    await act(() => new Promise(r => setTimeout(r, 450)));
    expect(text(form.querySelector('.picker__list'))).toContain('No other matches');
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    await act(() => fireEvent.input(input, { target: { value: 'zzzz' } }));
    await act(() => new Promise(r => setTimeout(r, 450)));
    expect(text(form.querySelector('.picker__list'))).toContain('No other matches');
  });

  it('picks the first match with Enter and closes with Escape', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok([])));
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Record a sale')));
    const form = container.querySelector('.form')!;
    const input = await openPicker(form);
    await act(() => fireEvent.input(input, { target: { value: 'bloom' } }));
    await act(() => fireEvent.keyDown(input, { key: 'Enter' }));
    expect(input.value).toBe('Bloomburrow — Play Booster');
    expect(form.querySelector('.picker__list')).toBeNull();
    await act(() => fireEvent.focus(input));
    await act(() => fireEvent.keyDown(input, { key: 'Escape' }));
    expect(form.querySelector('.picker__list')).toBeNull();
    await act(() => fireEvent.focus(input));
    await act(() => fireEvent.input(input, { target: { value: 'nothing like it' } }));
    expect(text(form.querySelector('.picker__list'))).toContain('No matches');
    await act(() => fireEvent.keyDown(input, { key: 'Enter' }));
    expect(input.value).toBe('nothing like it');
  });
});

describe('Finance sales', () => {
  it('records a sale and takes the packs out of stock', async () => {
    const created = { ...sale, id: 22, quantity: 4, proceeds: 30 };
    const fetchMock = vi.fn().mockResolvedValue(ok(created, 201));
    vi.stubGlobal('fetch', fetchMock);
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Record a sale')));
    const form = container.querySelector('.form')!;
    // Only packs in stock can be sold.
    await openPicker(form);
    expect(pickerItems(form)).toEqual(['Bloomburrow — Play Booster (12 in stock)']);

    await pick(form, 'Bloomburrow');
    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    await act(() => fireEvent.input(nums[0], { target: { value: '13' } }));
    expect(text(form)).toContain('Only 12 in stock.');
    expect(button(container, 'Record sale', form).disabled).toBe(true);
    await act(() => fireEvent.input(nums[0], { target: { value: '4' } }));
    await act(() => fireEvent.click(button(container, 'Each', form)));
    await act(() => fireEvent.input(nums[1], { target: { value: '7.5' } }));
    await act(() => fireEvent.input(nums[2], { target: { value: '2' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);

    expect(fetchMock.mock.calls[0][0]).toBe('/api/sales');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      packId: 1, quantity: 4, salePrice: 7.5, fees: 2, soldOn: localToday(),
    });
    expect(container.querySelectorAll('.tag--sale')).toHaveLength(2);
    await openPicker(form);
    expect(pickerItems(form).join()).toContain('(8 in stock)');
  });

  it('edits a sale and adjusts stock by the difference', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(ok({ ...sale, quantity: 5, proceeds: 30 })));
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Edit', rowOf(container, 'Sale'))));
    const form = container.querySelector('.form--inline')!;
    const nums = form.querySelectorAll<HTMLInputElement>('input[type=number]');
    expect([nums[0].value, nums[1].value, nums[2].value]).toEqual(['2', '12', '1']);
    await act(() => fireEvent.input(nums[0], { target: { value: '5' } }));
    await act(() => fireEvent.input(nums[1], { target: { value: '30' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);
    expect(text(rowOf(container, 'Sale'))).toContain('$30.00 for 5 packs');
    await act(() => fireEvent.click(button(container, 'Record a sale')));
    await openPicker(container.querySelector('.form')!);
    expect(pickerItems(container).join()).toContain('(9 in stock)');
  });

  it('undoes a sale, putting the packs back', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204 }));
    const { container } = setup();
    await act(() => fireEvent.click(rowOf(container, 'Sale').querySelector('.btn-row--danger')!));
    expect(toast.msg).toContain('The packs go back into stock');
    await act(async () => { toast.onConfirm!(); await flush(); });
    expect(container.querySelector('.tag--sale')).toBeNull();
    await act(() => fireEvent.click(button(container, 'Record a sale')));
    await openPicker(container.querySelector('.form')!);
    expect(pickerItems(container).join()).toContain('(14 in stock)');
  });

  it('keeps a sale when undoing fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: async () => '' }));
    const { container } = setup();
    await act(() => fireEvent.click(rowOf(container, 'Sale').querySelector('.btn-row--danger')!));
    await act(async () => { toast.onConfirm!(); await flush(); });
    expect(container.querySelector('.tag--sale')).not.toBeNull();
    expect(toast.msg).toBe('Failed to undo sale: 500');
  });
});

describe('Finance drafts', () => {
  it('corrects what was collected for a draft', async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok([{ ...draft, proceeds: 45 }]));
    vi.stubGlobal('fetch', fetchMock);
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Edit', rowOf(container, 'Draft'))));
    const form = container.querySelector('.form--inline')!;
    const input = form.querySelector<HTMLInputElement>('input[type=number]')!;
    expect(input.value).toBe('60');
    await act(() => fireEvent.input(input, { target: { value: '45' } }));
    await act(() => fireEvent.submit(form));
    await act(flush);
    expect(fetchMock.mock.calls[0][0]).toBe('/api/drafts/7/collected');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ amount: 45 });
    expect(text(rowOf(container, 'Draft'))).toContain('$45.00 collected');
    expect(container.querySelector('.form--inline')).toBeNull();
  });

  it('keeps the edit open when saving fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404, text: async () => 'draft not approved' }));
    const { container } = setup();
    await act(() => fireEvent.click(button(container, 'Edit', rowOf(container, 'Draft'))));
    await act(() => fireEvent.submit(container.querySelector('.form--inline')!));
    await act(flush);
    expect(toast.msg).toBe('Failed to save: draft not approved');
    expect(container.querySelector('.form--inline')).not.toBeNull();
  });
});

import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import Finance from './Finance.svelte';

// Page states the main Finance tests don't reach: empty pages, read-only
// viewers, single packs, drafts with no logged cost, and so on.

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

const pack = (id: number, extra: Record<string, unknown> = {}) =>
  ({ id, setName: `Set ${id}`, productType: 'Play Booster', quantity: 3, marketPrice: 5, ...extra });
const purchase = (extra: Record<string, unknown> = {}) => ({
  id: 10, packId: 1, setName: 'Set 1', productType: 'Play Booster', quantity: 1,
  totalPrice: 4, fees: 0, unitPrice: 4, currentPrice: 5, purchasedOn: '2026-10-01', ...extra,
});
const disposal = (extra: Record<string, unknown> = {}) => ({
  id: 20, packId: 1, setName: 'Set 1', productType: 'Play Booster', kind: 'sale', draftId: null,
  quantity: 1, proceeds: 6, marketValue: null, fees: 0, disposedOn: '2026-11-01', ...extra,
});
const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
const buttons = (c: HTMLElement) => Array.from(c.querySelectorAll('button')).map(b => text(b));

function setup(props: Record<string, unknown>) {
  return render(Finance, { props: { purchases: [], disposals: [], packs: [], canEdit: true, ...props } });
}

describe('Finance page states', () => {
  it('handles an empty page without positions or fee notes', () => {
    const { container } = setup({});
    expect(text(container.querySelector('.invested'))).not.toContain('tax & shipping');
    expect(container.querySelector('.positions, table')).toBeNull();
  });

  it('hides every edit control from viewers', () => {
    const { container } = setup({ canEdit: false, purchases: [purchase()], packs: [pack(1)] });
    expect(buttons(container).some(b => b.includes('Record a sale') || b.includes('Log a purchase'))).toBe(false);
  });

  it('opens and closes the sale form from its toggle', async () => {
    const { container } = setup({ purchases: [purchase()], packs: [pack(1)] });
    const toggle = () => Array.from(container.querySelectorAll('button')).find(b => b.textContent?.includes('Record a sale'))!;
    await act(() => fireEvent.click(toggle()));
    expect(container.querySelector('form')).not.toBeNull();
    await act(() => fireEvent.click(toggle()));
    expect(container.querySelector('form')).toBeNull();
  });

  it('shows a single-pack purchase and sale without per-pack breakdowns', () => {
    const { container } = setup({ purchases: [purchase()], disposals: [disposal()], packs: [pack(1)] });
    const log = text(container.querySelector('.log'));
    expect(log).toContain('$4.00 for 1 pack');
    expect(log).not.toContain(' each)');
    expect(log).toContain('$6.00 for 1 pack');
  });

  it('shows multi-pack purchases and sales with each-price and fees', () => {
    const { container } = setup({
      purchases: [purchase({ quantity: 4, totalPrice: 20, fees: 2, unitPrice: 5 })],
      disposals: [disposal({ quantity: 2, proceeds: 14, fees: 1 })],
      packs: [pack(1)],
    });
    const log = text(container.querySelector('.log'));
    expect(log).toContain('$20.00 for 4 packs');
    expect(log).toContain('($4.50 each)'); // fees excluded: (20 − 2) / 4
    expect(log).toContain('incl. $2.00 tax & shipping');
    expect(log).toContain('($7.00 each)');
    expect(log).toContain('$1.00 fees');
    expect(log).toContain('$14.00 for 2 packs');
  });

  it('says "no logged cost" for a draft of uncosted packs, without a market value', () => {
    const { container } = setup({
      packs: [pack(1), pack(2)],
      disposals: [disposal({ kind: 'draft', draftId: 7, packId: 2, setName: 'Set 2', quantity: 3, proceeds: 15, marketValue: 0 })],
    });
    const log = text(container.querySelector('.log'));
    expect(log).toContain('$15.00 collected');
    expect(log).not.toContain('market $');
    expect(log).toContain('no logged cost');
  });

  it('counts uncosted packs in a draft that also used costed ones', () => {
    const { container } = setup({
      packs: [pack(1), pack(2)],
      purchases: [purchase({ quantity: 3, totalPrice: 12, unitPrice: 4 })],
      disposals: [
        disposal({ id: 20, kind: 'draft', draftId: 7, packId: 1, quantity: 3, proceeds: 15, marketValue: 15 }),
        disposal({ id: 21, kind: 'draft', draftId: 7, packId: 2, setName: 'Set 2', quantity: 2, proceeds: 10, marketValue: 10 }),
      ],
    });
    expect(text(container.querySelector('.log'))).toContain('2 packs with no logged cost');
  });

  it('shows the market value of a draft when known', () => {
    const { container } = setup({
      purchases: [purchase({ quantity: 3, totalPrice: 12, unitPrice: 4 })],
      packs: [pack(1)],
      disposals: [disposal({ kind: 'draft', draftId: 7, quantity: 3, proceeds: 18, marketValue: 15 })],
    });
    expect(text(container.querySelector('.log'))).toContain('market $15.00');
  });

  it('lists positions sorted by set, including packs never bought', () => {
    const { container } = setup({
      packs: [pack(2, { setName: 'Beta' }), pack(1, { setName: 'Alpha' }), pack(3, { setName: 'Alpha', productType: 'Collector Booster' })],
      purchases: [purchase({ setName: 'Alpha' })],
    });
    const names = Array.from(container.querySelectorAll('tbody tr')).map(r => text(r));
    expect(names.length).toBeGreaterThan(0);
    expect(names[0]).toContain('Alpha');
  });
});

describe('Finance holdings sorting', () => {
  const rows = (c: HTMLElement) => Array.from(c.querySelectorAll('.holdings tbody tr')).map(r => r.querySelector('.holdings__name')?.childNodes[0]?.textContent?.trim());
  const header = (c: HTMLElement, label: string) =>
    Array.from(c.querySelectorAll<HTMLButtonElement>('.holdings th button')).find(b => b.textContent?.startsWith(label))!;
  const props = {
    packs: [pack(1, { setName: 'Cheap', marketPrice: 10 }), pack(2, { setName: 'Dear', marketPrice: 12 })],
    purchases: [
      purchase({ id: 1, packId: 1, setName: 'Cheap', quantity: 3, totalPrice: 6, unitPrice: 2 }),   // +$24, +400%
      purchase({ id: 2, packId: 2, setName: 'Dear', quantity: 3, totalPrice: 30, unitPrice: 10 }),  // +$6, +20%
    ],
  };

  afterEach(() => localStorage.clear());

  it('sorts by avg cost, $ and % gain, and cycles back to the default order', async () => {
    localStorage.clear();
    const { container } = setup(props);
    const initial = rows(container);

    await act(() => fireEvent.click(header(container, 'Avg cost')));
    expect(rows(container)).toEqual(['Dear', 'Cheap']);
    expect(header(container, 'Avg cost').textContent).toContain('↓');
    expect(header(container, 'Avg cost').closest('th')?.getAttribute('aria-sort')).toBe('descending');

    await act(() => fireEvent.click(header(container, 'Avg cost')));
    expect(rows(container)).toEqual(['Cheap', 'Dear']);

    await act(() => fireEvent.click(header(container, 'Avg cost')));
    expect(rows(container)).toEqual(initial);

    await act(() => fireEvent.click(header(container, '%')));
    expect(rows(container)).toEqual(['Cheap', 'Dear']);
    await act(() => fireEvent.click(header(container, 'Unrealized')));
    expect(rows(container)).toEqual(['Cheap', 'Dear']);
    await act(() => fireEvent.click(header(container, 'Realized')));
    expect(header(container, 'Realized').closest('th')?.getAttribute('aria-sort')).toBe('descending');
  });

  it('sorts by pack name A–Z and back, and marks sortable headers', async () => {
    localStorage.clear();
    const { container } = setup({ ...props, packs: [pack(2, { setName: 'Dear', marketPrice: 12 }), pack(1, { setName: 'Cheap', marketPrice: 10 })] });
    // Every sortable header shows the neutral icon until used.
    const icons = Array.from(container.querySelectorAll('.holdings th .sort__icon')).map(i => i.textContent);
    expect(icons).toEqual(['⇅', '⇅', '⇅', '⇅', '⇅']);

    await act(() => fireEvent.click(header(container, 'Pack')));
    expect(rows(container)).toEqual(['Cheap', 'Dear']);
    expect(header(container, 'Pack').classList.contains('sort--active')).toBe(true);
    expect(header(container, 'Pack').textContent).toContain('↑');

    await act(() => fireEvent.click(header(container, 'Pack')));
    expect(rows(container)).toEqual(['Dear', 'Cheap']);
    expect(header(container, 'Pack').textContent).toContain('↓');
  });

  it('remembers the sort across reloads and ignores broken storage', async () => {
    localStorage.clear();
    const first = setup(props);
    await act(() => fireEvent.click(header(first.container, 'Avg cost')));
    first.unmount();
    expect(rows(setup(props).container)).toEqual(['Dear', 'Cheap']);

    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    const blocked = setup(props);
    await act(() => fireEvent.click(header(blocked.container, 'Avg cost')));
    expect(rows(blocked.container)).toEqual(['Dear', 'Cheap']);
  });
});

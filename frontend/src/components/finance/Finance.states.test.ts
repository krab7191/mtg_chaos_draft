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

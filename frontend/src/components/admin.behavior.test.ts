import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act } from '../tests/svelte';
import WeightsList from './weights/WeightsList.svelte';
import CollectionList from './collection/CollectionList.svelte';
import DraftsList from './drafts/DraftsList.svelte';
import { toast } from '../lib/toast.svelte';

const flush = () => new Promise(r => setTimeout(r, 0));

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, statusText: 'OK', text: async () => '' }));
  // Run confirm callbacks immediately, as if the admin clicked "yes".
  vi.spyOn(toast, 'confirm').mockImplementation((_msg: string, onConfirm: () => void) => { onConfirm(); });
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

// ── WeightsList ───────────────────────────────────────────────────────────────

describe('WeightsList behavior', () => {
  const settings = { priceFloor: 0, priceCap: 0, quantityCap: 0, packWeights: {} as Record<string, number> };
  const wp = (id: number, setName: string, extra: Record<string, unknown> = {}) =>
    ({ id, setName, productType: 'Draft Booster', marketPrice: 5, quantity: 10, cardsPerPack: 15, ...extra });

  const oddsLabels = (c: HTMLElement) =>
    Array.from(c.querySelectorAll('.odds__label')).map(e => e.textContent ?? '');
  const setNames = (c: HTMLElement) =>
    Array.from(c.querySelectorAll('.set-group__name')).map(e => e.textContent ?? '');
  const sortBtn = (c: HTMLElement, i: number) => c.querySelectorAll('.sort-btn')[i] as HTMLButtonElement;

  async function typeInto(c: HTMLElement, sel: string, value: string) {
    const el = c.querySelector(sel) as HTMLInputElement;
    el.value = value;
    await act(() => fireEvent.input(el));
  }

  it('floor and cap clamp prices; junk input resets to no limit', async () => {
    const { container } = render(WeightsList, {
      props: { packs: [wp(1, 'Cheap', { marketPrice: 1 }), wp(2, 'Pricey', { marketPrice: 9 })], settings },
    });
    expect(oddsLabels(container)).toEqual(['90.0%', '10.0%']);

    await typeInto(container, '#price-floor', '5');
    await typeInto(container, '#price-cap', '5');
    expect(oddsLabels(container)).toEqual(['50.0%', '50.0%']);

    await typeInto(container, '#price-floor', 'abc');
    await typeInto(container, '#price-cap', '-3');
    expect(oddsLabels(container)).toEqual(['90.0%', '10.0%']);
  });

  it('quantity cap evens out heavily stocked packs', async () => {
    const { container } = render(WeightsList, {
      props: { packs: [wp(1, 'Many', { quantity: 40 }), wp(2, 'Few', { quantity: 4 })], settings },
    });
    await typeInto(container, '#qty-cap', '4');
    expect(oddsLabels(container)).toEqual(['50.0%', '50.0%']);
    await typeInto(container, '#qty-cap', 'x');
    expect(oddsLabels(container)).toEqual(['9.1%', '90.9%']); // rows sorted by name: Few, Many
  });

  it('starts from saved caps and pack weights', () => {
    const saved = { priceFloor: 3, priceCap: 4, quantityCap: 2, packWeights: { '1': 1 } };
    const { container } = render(WeightsList, { props: { packs: [wp(1, 'A'), wp(2, 'B')], settings: saved } });
    expect((container.querySelector('#price-floor') as HTMLInputElement).value).toBe('3');
    expect(oddsLabels(container)).toEqual(['66.7%', '33.3%']);
  });

  it('handles a missing packWeights setting', () => {
    const noWeights = { priceFloor: 0, priceCap: 0, quantityCap: 0 } as typeof settings;
    const { container } = render(WeightsList, { props: { packs: [wp(1, 'A')], settings: noWeights } });
    expect(oddsLabels(container)).toEqual(['100.0%']);
  });

  it('small packs count by draft slots, unpriced packs use the average price', () => {
    const packs = [
      wp(1, 'Normal', { quantity: 3 }),
      wp(2, 'Epilogue', { quantity: 9, cardsPerPack: 5 }),   // 3 slots
      wp(3, 'Old', { quantity: 6, cardsPerPack: 8 }),        // 3 slots
      wp(4, 'Unknown', { quantity: 3, cardsPerPack: undefined, marketPrice: null }),
      wp(5, 'Empty', { quantity: 1, cardsPerPack: 5 }),      // 0 slots
    ];
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(oddsLabels(container)).toEqual(['0.0%', '25.0%', '25.0%', '25.0%', '25.0%']);
  });

  it('everything unpriced or empty still renders', () => {
    const packs = [wp(1, 'A', { marketPrice: null }), wp(2, 'B', { quantity: 0 })];
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(oddsLabels(container)).toEqual(['100.0%', '0.0%']);
    const empty = render(WeightsList, { props: { packs: [wp(3, 'C', { quantity: 0 })], settings } });
    expect(oddsLabels(empty.container)).toEqual(['0.0%']);
  });

  it('sorts by name, price and odds in both directions', async () => {
    const packs = [wp(1, 'Bravo', { marketPrice: 2 }), wp(2, 'Alpha', { marketPrice: 8 }), wp(3, 'Charlie', { marketPrice: null })];
    const { container } = render(WeightsList, { props: { packs, settings } });
    expect(setNames(container)).toEqual(['Alpha', 'Bravo', 'Charlie']);

    await act(() => fireEvent.click(sortBtn(container, 0)));
    expect(setNames(container)).toEqual(['Charlie', 'Bravo', 'Alpha']);

    await act(() => fireEvent.click(sortBtn(container, 1))); // price desc
    expect(setNames(container)).toEqual(['Alpha', 'Bravo', 'Charlie']);
    await act(() => fireEvent.click(sortBtn(container, 1))); // price asc
    expect(setNames(container)).toEqual(['Charlie', 'Bravo', 'Alpha']);

    await act(() => fireEvent.click(sortBtn(container, 2))); // odds desc
    expect(setNames(container)[0]).toBe('Bravo');
    expect(sortBtn(container, 2).textContent).toContain('↓');
  });

  it('keeps rows in place while adjusting weights in odds order, then re-sorts', async () => {
    vi.useFakeTimers();
    try {
      const packs = [wp(1, 'A', { marketPrice: 2 }), wp(2, 'B', { marketPrice: 4 })];
      const { container } = render(WeightsList, { props: { packs, settings } });
      await act(() => fireEvent.click(sortBtn(container, 2))); // odds desc: A, B
      expect(setNames(container)).toEqual(['A', 'B']);

      const bPlus = container.querySelectorAll('.weight-btn')[3] as HTMLButtonElement; // B "+"
      for (let i = 0; i < 15; i++) await act(() => fireEvent.click(bPlus));
      expect(setNames(container)).toEqual(['A', 'B']); // frozen while editing

      await act(() => vi.advanceTimersByTime(600));
      expect(setNames(container)).toEqual(['B', 'A']); // re-sorted after the pause
    } finally {
      vi.useRealTimers();
    }
  });

  it('Save reports success and failure', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(WeightsList, { props: { packs: [wp(1, 'A')], settings } });
    await act(() => fireEvent.click(container.querySelector('.btn-save')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Settings saved', 'success');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, statusText: 'Boom' }));
    await act(() => fireEvent.click(container.querySelector('.btn-save')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to save: 500 Boom', 'error');
  });
});

// ── CollectionList ────────────────────────────────────────────────────────────

describe('CollectionList behavior', () => {
  const cp = (id: number, setName: string, extra: Record<string, unknown> = {}) =>
    ({ id, setName, productType: 'Draft Booster', marketPrice: 5, quantity: 2, cardsPerPack: 15, ...extra });
  const setNames = (c: HTMLElement) =>
    Array.from(c.querySelectorAll('.set-group__name')).map(e => e.textContent ?? '');

  it('shows total value, skipping unpriced packs, and a footnote for small packs', () => {
    const packs = [cp(1, 'A', { marketPrice: 2.5, quantity: 4 }), cp(2, 'B', { marketPrice: null, cardsPerPack: 5 })];
    const { container } = render(CollectionList, { props: { packs } });
    expect(container.querySelector('.collection__header')?.textContent).toContain('$10.00');
    expect(container.querySelector('.collection__footnote')).not.toBeNull();
  });

  it('hides totals when there is nothing priced or in stock', () => {
    const { container } = render(CollectionList, { props: { packs: [cp(1, 'A', { marketPrice: null, quantity: 0 })] } });
    expect(container.querySelectorAll('.collection__value')).toHaveLength(0);
  });

  it('sorts by name, quantity and price', async () => {
    const packs = [cp(1, 'Bravo', { quantity: 9, marketPrice: 1 }), cp(2, 'Alpha', { quantity: 1, marketPrice: 9 })];
    const { container } = render(CollectionList, { props: { packs } });
    const [nameBtn, qtyBtn, priceBtn] = Array.from(container.querySelectorAll('.sort-btn')) as HTMLButtonElement[];

    await act(() => fireEvent.click(nameBtn));
    expect(setNames(container)).toEqual(['Bravo', 'Alpha']);
    expect(nameBtn.textContent).toContain('↓');

    await act(() => fireEvent.click(qtyBtn));
    expect(setNames(container)).toEqual(['Bravo', 'Alpha']);
    await act(() => fireEvent.click(qtyBtn));
    expect(setNames(container)).toEqual(['Alpha', 'Bravo']);

    await act(() => fireEvent.click(priceBtn));
    expect(setNames(container)).toEqual(['Alpha', 'Bravo']);
  });

  it('changes quantity, and rolls back with an error when the save fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(CollectionList, { props: { packs: [cp(1, 'A', { quantity: 2 })] } });
    const qty = () => container.querySelector('.qty__val')?.textContent;

    await act(() => fireEvent.click(container.querySelector('.qty__btn--inc')!));
    await act(flush);
    expect(qty()).toBe('3');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 409, statusText: 'Conflict' }));
    await act(() => fireEvent.click(container.querySelector('.qty__btn--dec')!));
    await act(flush);
    expect(qty()).toBe('3');
    expect(show).toHaveBeenCalledWith('Failed to update quantity: 409 Conflict', 'error');
  });

  it('deletes a pack, and keeps it with an error when the delete fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(CollectionList, { props: { packs: [cp(1, 'A'), cp(2, 'B')] } });

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, statusText: 'Err' }));
    await act(() => fireEvent.click(container.querySelector('.btn-delete')!));
    await act(flush);
    expect(setNames(container)).toEqual(['A', 'B']);
    expect(show).toHaveBeenCalledWith('Failed to delete: 500 Err', 'error');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
    await act(() => fireEvent.click(container.querySelector('.btn-delete')!));
    await act(flush);
    expect(setNames(container)).toEqual(['B']);
  });
});

// ── DraftsList ────────────────────────────────────────────────────────────────

describe('DraftsList behavior', () => {
  const mk = (id: number, extra: Record<string, unknown> = {}) => ({
    id, draftedAt: '2026-11-10T20:00:00Z', approvedAt: null, approvedBy: null, podSize: 2,
    picks: [
      { id: id * 10, packId: 1, setName: 'A', productType: 'Draft Booster', marketPrice: 6 },
      { id: id * 10 + 1, packId: 2, setName: 'B', productType: 'Draft Booster', marketPrice: null },
    ],
    ...extra,
  });

  it('shows pack total and per-player cost; older drafts default to 4 players', () => {
    const { container } = render(DraftsList, { props: { drafts: [mk(1), mk(2, { podSize: undefined })], isAdmin: false } });
    const costs = Array.from(container.querySelectorAll('.draft-cost')).map(e => e.textContent ?? '');
    expect(costs[0]).toContain('2 players · 2 packs · $6.00');
    expect(costs[0]).toContain('$3.00 / player');
    expect(costs[1]).toContain('4 players');
    expect(costs[1]).toContain('$1.50 / player');
    expect(container.querySelector('.btn-approve')).toBeNull(); // not admin
  });

  it('approving marks only that draft approved', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(DraftsList, { props: { drafts: [mk(1), mk(2)], isAdmin: true } });
    await act(() => fireEvent.click(container.querySelector('.btn-approve')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Draft approved', 'success');
    expect(container.querySelectorAll('.badge--approved')).toHaveLength(1);
    expect(container.querySelectorAll('.btn-approve')).toHaveLength(1);
  });

  it('shows the server message, or the status, when approval fails', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(DraftsList, { props: { drafts: [mk(1)], isAdmin: true } });

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 409, text: async () => 'insufficient stock for: A\n' }));
    await act(() => fireEvent.click(container.querySelector('.btn-approve')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('insufficient stock for: A', 'error');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, text: async () => '  ' }));
    await act(() => fireEvent.click(container.querySelector('.btn-approve')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to approve: 500', 'error');

    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    await act(() => fireEvent.click(container.querySelector('.btn-approve')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to approve draft', 'error');
    expect((container.querySelector('.btn-approve') as HTMLButtonElement).disabled).toBe(false);
  });

  it('removes a draft, or reports why it could not', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = render(DraftsList, { props: { drafts: [mk(1), mk(2)], isAdmin: true } });

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    await act(() => fireEvent.click(container.querySelector('.btn-remove')!));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to remove draft: 500', 'error');
    expect(container.querySelectorAll('.draft-card')).toHaveLength(2);

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
    await act(() => fireEvent.click(container.querySelector('.btn-remove')!));
    await act(flush);
    expect(container.querySelectorAll('.draft-card')).toHaveLength(1);
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act } from '../tests/svelte';
import PackSelector from './PackSelector.svelte';
import { toast } from '../lib/toast.svelte';

type Pack = {
  id: number; name: string; setName: string; productType: string;
  quantity: number; cardsPerPack: number; marketPrice?: number | null;
};

const baseSettings = { priceFloor: 0, priceCap: 0, quantityCap: 0, packWeights: {} as Record<string, number> };

const pack = (id: number, setName: string, extra: Partial<Pack> = {}): Pack => ({
  id, name: setName, setName, productType: 'Draft Booster', quantity: 20, cardsPerPack: 15, marketPrice: 5, ...extra,
});

const flush = () => new Promise(r => setTimeout(r, 0));

function button(container: HTMLElement, text: string) {
  return Array.from(container.querySelectorAll('button')).find(b => b.textContent?.includes(text)) as HTMLButtonElement;
}

function oddsFor(container: HTMLElement, setName: string): string {
  const item = Array.from(container.querySelectorAll('.pack-item')).find(
    li => li.querySelector('.pack-item__name')?.textContent === setName,
  );
  return item?.querySelector('.pack-item__odds')?.textContent?.trim() ?? '';
}

function names(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('.pack-item__name')).map(n => n.textContent ?? '');
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

// ── Odds / weighting ──────────────────────────────────────────────────────────

describe('PackSelector odds', () => {
  it('cheaper packs get better odds', () => {
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Cheap', { marketPrice: 2 }), pack(2, 'Pricey', { marketPrice: 8 })], settings: baseSettings },
    });
    expect(oddsFor(container, 'Cheap')).toBe('80.0%');
    expect(oddsFor(container, 'Pricey')).toBe('20.0%');
  });

  it('price floor and cap clamp prices before weighting', () => {
    const settings = { ...baseSettings, priceFloor: 4, priceCap: 4 };
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Cheap', { marketPrice: 1 }), pack(2, 'Pricey', { marketPrice: 50 })], settings },
    });
    // Both clamp to $4, so equal odds.
    expect(oddsFor(container, 'Cheap')).toBe('50.0%');
    expect(oddsFor(container, 'Pricey')).toBe('50.0%');
  });

  it('quantity cap limits how much stock boosts odds', () => {
    const settings = { ...baseSettings, quantityCap: 5 };
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Many', { quantity: 50 }), pack(2, 'Few', { quantity: 5 })], settings },
    });
    expect(oddsFor(container, 'Many')).toBe('50.0%');
  });

  it('unpriced packs are weighted at the average price', () => {
    const { container } = render(PackSelector, {
      props: {
        packs: [pack(1, 'A', { marketPrice: 4 }), pack(2, 'B', { marketPrice: 4 }), pack(3, 'Unpriced', { marketPrice: null })],
        settings: baseSettings,
      },
    });
    expect(oddsFor(container, 'Unpriced')).toBe('33.3%');
  });

  it('per-pack weight overrides shift odds', () => {
    const settings = { ...baseSettings, packWeights: { '1': 1 } }; // +100%
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Boosted'), pack(2, 'Normal')], settings },
    });
    expect(oddsFor(container, 'Boosted')).toBe('66.7%');
  });

  it('missing packWeights setting is treated as no overrides', () => {
    const settings = { priceFloor: 0, priceCap: 0, quantityCap: 0 } as typeof baseSettings;
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A'), pack(2, 'B')], settings } });
    expect(oddsFor(container, 'A')).toBe('50.0%');
  });

  it('packs with no full slot left get 0% odds and are dimmed', () => {
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Normal'), pack(2, 'OldSingle', { quantity: 1, cardsPerPack: 8 })], settings: baseSettings },
    });
    expect(oddsFor(container, 'OldSingle')).toBe('0.0%');
    expect(container.querySelector('.pack-item__label--depleted')).not.toBeNull();
  });

  it('deselected packs show no odds', async () => {
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'A'), pack(2, 'B')], settings: baseSettings },
    });
    const hide = Array.from(container.querySelectorAll('label.toggle-label input'))[0] as HTMLInputElement;
    await act(() => fireEvent.click(hide)); // show deselected
    const box = container.querySelector('.pack-checkbox') as HTMLInputElement;
    await act(() => fireEvent.change(box));
    expect(oddsFor(container, 'A')).toBe('—');
    expect(oddsFor(container, 'B')).toBe('100.0%');
  });
});

// ── Selection & sorting ───────────────────────────────────────────────────────

describe('PackSelector selection and sorting', () => {
  it('deselect all / select all toggle the pool and the pick button', async () => {
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A'), pack(2, 'B')], settings: baseSettings } });
    await act(() => fireEvent.click(button(container, 'Deselect all')));
    expect(container.querySelector('.count-pill')?.textContent).toContain('0 selected');
    expect(button(container, 'Pick a Pack').disabled).toBe(true);
    expect(names(container)).toHaveLength(0); // hidden while "hide deselected" is on

    await act(() => fireEvent.click(button(container, 'Select all')));
    expect(container.querySelector('.count-pill')?.textContent).toContain('2 selected');
    expect(button(container, 'Select all').disabled).toBe(true);
  });

  it('re-checking a pack adds it back', async () => {
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A'), pack(2, 'B')], settings: baseSettings } });
    const hide = container.querySelector('label.toggle-label input') as HTMLInputElement;
    await act(() => fireEvent.click(hide));
    const box = container.querySelector('.pack-checkbox') as HTMLInputElement;
    await act(() => fireEvent.change(box));
    await act(() => fireEvent.change(box));
    expect(container.querySelector('.count-pill')?.textContent).toContain('2 selected');
  });

  it('sorts by name both ways and by price both ways', async () => {
    const packs = [pack(1, 'Bravo', { marketPrice: 9 }), pack(2, 'Alpha', { marketPrice: 3 }), pack(3, 'Charlie', { marketPrice: null })];
    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    const [nameBtn, , priceBtn] = Array.from(container.querySelectorAll('.sort-btn')) as HTMLButtonElement[];

    expect(names(container)).toEqual(['Alpha', 'Bravo', 'Charlie']);
    expect(nameBtn.textContent).toContain('↑');

    await act(() => fireEvent.click(nameBtn)); // name desc
    expect(names(container)).toEqual(['Charlie', 'Bravo', 'Alpha']);

    await act(() => fireEvent.click(priceBtn)); // price desc first
    expect(names(container)).toEqual(['Bravo', 'Alpha', 'Charlie']);
    expect(priceBtn.textContent).toContain('↓');

    await act(() => fireEvent.click(priceBtn)); // price asc
    expect(names(container)).toEqual(['Charlie', 'Alpha', 'Bravo']);

    await act(() => fireEvent.click(nameBtn)); // back to name asc
    expect(names(container)).toEqual(['Alpha', 'Bravo', 'Charlie']);
  });

  it('ties on set name sort by product type', () => {
    const packs = [pack(1, 'Same', { productType: 'Set Booster' }), pack(2, 'Same', { productType: 'Draft Booster' })];
    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    const types = Array.from(container.querySelectorAll('.pack-item__meta')).map(m => m.textContent ?? '');
    expect(types[0]).toContain('Draft Booster');
  });

  it('marks small packs and explains slots in the footnote, even without a price', () => {
    const packs = [pack(1, 'Normal'), pack(2, 'Old', { cardsPerPack: 8, marketPrice: null })];
    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    const meta = Array.from(container.querySelectorAll('.pack-item__meta')).map(m => m.textContent ?? '');
    expect(meta.some(m => m.includes('*'))).toBe(true);
    const foot = container.querySelector('.pack-footnote')?.textContent ?? '';
    expect(foot).toContain('2 packs per slot');
    expect(foot).not.toContain('=');
  });

  it('shows a dash for packs without a price', () => {
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A', { marketPrice: null })], settings: baseSettings } });
    expect(container.querySelector('.pack-item__price')?.textContent).toBe('—');
  });
});

// ── Quantity sort ─────────────────────────────────────────────────────────────

describe('PackSelector quantity sort', () => {
  it('sorts by available slots, most first, then least first; ties by name', async () => {
    const packs = [
      pack(1, 'Few', { quantity: 2 }),
      pack(2, 'Many', { quantity: 9 }),
      pack(3, 'Small', { quantity: 9, cardsPerPack: 5 }), // 3 slots
      pack(4, 'Also Few', { quantity: 2 }),
    ];
    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    const qtyBtn = container.querySelectorAll('.sort-btn')[1] as HTMLButtonElement;
    expect(qtyBtn.textContent).toBe('Qty');

    await act(() => fireEvent.click(qtyBtn));
    expect(names(container)).toEqual(['Many', 'Small', 'Also Few', 'Few']);
    expect(qtyBtn.textContent).toContain('↓');

    await act(() => fireEvent.click(qtyBtn));
    expect(names(container)).toEqual(['Also Few', 'Few', 'Small', 'Many']);
  });
});

// ── Persistence across reloads ────────────────────────────────────────────────

describe('PackSelector remembers state across reloads', () => {
  it('keeps unchecked packs, sort, and toggles after a reload', async () => {
    const packs = [pack(1, 'A'), pack(2, 'B'), pack(3, 'C')];
    const first = render(PackSelector, { props: { packs, settings: baseSettings } });
    const c = first.container;
    await act(() => fireEvent.click(c.querySelector('label.toggle-label input')!)); // show deselected
    await act(() => fireEvent.change(c.querySelectorAll('.pack-checkbox')[1])); // uncheck B
    await act(() => fireEvent.click(c.querySelectorAll('.sort-btn')[2]));       // price sort
    first.unmount();

    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    expect(container.querySelector('.count-pill')?.textContent).toContain('2 selected');
    expect((container.querySelectorAll('.sort-btn')[2] as HTMLElement).classList.contains('sort-btn--active')).toBe(true);
    expect((container.querySelector('label.toggle-label input') as HTMLInputElement).checked).toBe(false);
    const bBox = Array.from(container.querySelectorAll('.pack-item')).find(li => li.querySelector('.pack-item__name')?.textContent === 'B')!
      .querySelector('.pack-checkbox') as HTMLInputElement;
    expect(bBox.checked).toBe(false);
  });

  it('new packs start selected, and choices for packs not shown are kept', () => {
    localStorage.setItem('pack-selector-prefs', JSON.stringify({ unchecked: ['2', '99'] }));
    const { container, unmount } = render(PackSelector, {
      props: { packs: [pack(1, 'A'), pack(2, 'B'), pack(3, 'New')], settings: baseSettings },
    });
    expect(names(container)).toEqual(['A', 'New']);
    unmount();
    const saved = JSON.parse(localStorage.getItem('pack-selector-prefs')!);
    expect(saved.unchecked.sort()).toEqual(['2', '99']);
  });

  it('stock stays correct after a reload mid-draft', () => {
    localStorage.setItem('draft-history', JSON.stringify([
      { key: 'a', packId: 1, setName: 'Last', productType: 'Draft Booster', marketPrice: 5 },
      { key: 'b', packId: 1, setName: 'Last', productType: 'Draft Booster', marketPrice: 5 },
    ]));
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Last', { quantity: 3 }), pack(2, 'Other')], settings: baseSettings },
    });
    const meta = Array.from(container.querySelectorAll('.pack-item__meta')).map(m => m.textContent);
    expect(meta[0]).toContain('(1)'); // 3 in stock − 2 already picked
  });

  it('a saved draft stays locked after a reload', () => {
    localStorage.setItem('draft-history', JSON.stringify([{ key: 'a', packId: 1, setName: 'A', productType: 'X', marketPrice: 5 }]));
    localStorage.setItem('pack-selector-prefs', JSON.stringify({ draftSaved: true }));
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    expect((container.querySelector('#pod-size') as HTMLInputElement).disabled).toBe(true);
  });

  it('ignores corrupt saved preferences', () => {
    localStorage.setItem('pack-selector-prefs', 'nope');
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    expect(container.querySelector('.count-pill')?.textContent).toContain('1 selected');
  });
});

// ── Recording toggle ──────────────────────────────────────────────────────────

describe('PackSelector record draft toggle', () => {
  it('with recording off, picks are not kept and Roll all is unavailable', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    const record = container.querySelector('.history-toggle input') as HTMLInputElement;
    await act(() => fireEvent.change(record, { target: { checked: false } }));

    const roll = button(container, 'Roll all');
    expect(roll.disabled).toBe(true);
    expect(roll.title).toContain('Record draft');

    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    expect(container.querySelector('.toast')?.textContent).toContain('A');
    expect(container.querySelector('.history')).toBeNull();
    expect(fetch).not.toHaveBeenCalled();
  });

  it('turning recording back on resumes recording', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    const record = container.querySelector('.history-toggle input') as HTMLInputElement;
    await act(() => fireEvent.change(record, { target: { checked: false } }));
    await act(() => fireEvent.change(record, { target: { checked: true } }));
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    expect(container.querySelectorAll('.history__item')).toHaveLength(1);
  });
});

// ── Picking details ───────────────────────────────────────────────────────────

describe('PackSelector picking', () => {
  it('clicking the result toast dismisses it, and it auto-dismisses', async () => {
    vi.useFakeTimers();
    try {
      vi.spyOn(Math, 'random').mockReturnValue(0);
      const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
      await act(() => fireEvent.click(button(container, 'Pick a Pack')));
      expect(container.querySelector('.toast__text')?.textContent).toBe('A · Draft Booster');
      await act(() => fireEvent.click(container.querySelector('.toast')!));
      expect(container.querySelector('.toast')).toBeNull();

      await act(() => fireEvent.click(button(container, 'Pick a Pack')));
      await act(() => fireEvent.click(button(container, 'Pick a Pack'))); // restarts the timer
      await act(() => vi.advanceTimersByTime(3100));
      expect(container.querySelector('.toast')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('shows progress while a draft is in progress', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    expect(container.querySelector('.draft-full')?.textContent).toBe('1/12');
    expect(container.querySelector('.history__title')?.textContent).toContain('1/12');
  });

  it('a pack that runs out is unchecked automatically', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const packs = [pack(1, 'Last', { quantity: 1 }), pack(2, 'Plenty')];
    const { container } = render(PackSelector, { props: { packs, settings: baseSettings } });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    expect(names(container)).toEqual(['Plenty']);
  });

  it('keeps the stored price and notes unpriced picks when the live price fails', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    let resolve!: (v: unknown) => void;
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(new Promise(r => { resolve = r; })));
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Unpriced', { marketPrice: null })], settings: baseSettings },
    });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    expect(container.querySelector('.cost__note')?.textContent).toContain('Fetching');
    resolve({ ok: false });
    await act(flush);
    expect(container.querySelector('.cost__note')?.textContent).toContain('1 pick without a price');
    expect(container.querySelector('.history__pack-price')?.textContent?.trim()).toBe('—');
  });

  it('plural note for several unpriced picks; network errors are tolerated', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    const { container } = render(PackSelector, {
      props: { packs: [pack(1, 'Unpriced', { marketPrice: null })], settings: baseSettings },
    });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    await act(flush);
    expect(container.querySelector('.cost__note')?.textContent).toContain('2 picks without a price');
  });

  it('ignores a live price response without a numeric price', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ marketPrice: null }) }));
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A', { marketPrice: 5 })], settings: baseSettings } });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    await act(flush);
    expect(container.querySelector('.history__pack-price')?.textContent?.trim()).toBe('$5.00');
  });

  it('clearing history while a price is loading drops the late result', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    let resolve!: (v: unknown) => void;
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(new Promise(r => { resolve = r; })));
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    await act(() => fireEvent.click(container.querySelector('.history__clear')!));
    expect(container.querySelector('.history')).toBeNull();
    resolve({ ok: true, json: async () => ({ marketPrice: 9 }) });
    await act(flush);
    expect(container.querySelector('.history')).toBeNull();
  });

  it('warns when even the whole collection cannot fill the draft', () => {
    localStorage.setItem('draft-pod-size', '8');
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A', { quantity: 3 })], settings: baseSettings } });
    expect(container.querySelector('.stock-warning')?.textContent).toContain('whole collection only fills 3');
  });
});

// ── Saving ────────────────────────────────────────────────────────────────────

describe('PackSelector saving', () => {
  function fillDraft(fetchImpl: ReturnType<typeof vi.fn>) {
    vi.stubGlobal('fetch', fetchImpl);
    localStorage.setItem('draft-pod-size', '2');
    return render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
  }

  it('reports a failed save and lets the pod size be changed again', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = fillDraft(vi.fn(async (url: string) =>
      url === '/api/drafts' ? { ok: false } : { ok: true, json: async () => ({ marketPrice: 5 }) }));
    await act(() => fireEvent.click(button(container, 'Roll all')));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to save draft', 'error');
    expect((container.querySelector('#pod-size') as HTMLInputElement).disabled).toBe(false);
  });

  it('reports a save that errors on the network', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = fillDraft(vi.fn(async (url: string) => {
      if (url === '/api/drafts') throw new Error('offline');
      return { ok: true, json: async () => ({ marketPrice: 5 }) };
    }));
    await act(() => fireEvent.click(button(container, 'Roll all')));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Failed to save draft', 'error');
  });

  it('a successful save locks the pod size and toasts', async () => {
    const show = vi.spyOn(toast, 'show');
    const { container } = fillDraft(vi.fn(async () => ({ ok: true, json: async () => ({ marketPrice: 5 }) })));
    await act(() => fireEvent.click(button(container, 'Roll all')));
    await act(flush);
    expect(show).toHaveBeenCalledWith('Draft saved', 'success');
    expect((container.querySelector('#pod-size') as HTMLInputElement).disabled).toBe(true);
    expect(container.querySelector('.draft-full')?.textContent).toContain('Draft complete (6/6)');
  });

  it('Roll all with one slot left says "1 pack"', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    localStorage.setItem('draft-pod-size', '2');
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    for (let i = 0; i < 5; i++) await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    await act(() => fireEvent.click(button(container, 'Roll all')));
    expect(container.querySelector('.toast__text')?.textContent).toBe('Rolled 1 pack');
  });
});

// ── Pod size & restored state ─────────────────────────────────────────────────

describe('PackSelector pod size and restore', () => {
  it('clamps pod size to the allowed range and ignores junk input', async () => {
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A', { quantity: 99 })], settings: baseSettings } });
    const input = container.querySelector('#pod-size') as HTMLInputElement;

    input.value = '99';
    await act(() => fireEvent.change(input));
    expect(input.value).toBe('16');

    input.value = '1';
    await act(() => fireEvent.change(input));
    expect(input.value).toBe('2');

    input.value = '';
    await act(() => fireEvent.change(input));
    expect(input.value).toBe('2');
  });

  it('cannot shrink the pod below the picks already made', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    for (let i = 0; i < 7; i++) await act(() => fireEvent.click(button(container, 'Pick a Pack')));
    const input = container.querySelector('#pod-size') as HTMLInputElement;
    input.value = '2';
    await act(() => fireEvent.change(input));
    expect(input.value).toBe('3'); // 7 picks need at least 3 players
  });

  it('ignores an invalid stored pod size', () => {
    localStorage.setItem('draft-pod-size', 'banana');
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    expect((container.querySelector('#pod-size') as HTMLInputElement).value).toBe('4');
  });

  it('restores history from before a reload, including older entries', () => {
    localStorage.setItem('draft-history', JSON.stringify([
      { packId: 1, setName: 'Old Entry', productType: 'Draft Booster', marketPrice: 4, price: '$4.00' },
      { key: 'k', packId: 2, setName: 'Small', productType: 'Booster', unitPrice: 2, packCount: 3, marketPrice: 6, pricing: true },
    ]));
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    const items = container.querySelectorAll('.history__item');
    expect(items).toHaveLength(2);
    expect(items[1].textContent).toContain('3× $2.00');
    expect(container.querySelector('.cost')?.textContent).toContain('$10.00');
    expect(container.querySelector('.cost__note')).toBeNull(); // nothing still "fetching" after reload
  });

  it('survives corrupt stored history', () => {
    localStorage.setItem('draft-history', '{not json');
    const { container } = render(PackSelector, { props: { packs: [pack(1, 'A')], settings: baseSettings } });
    expect(container.querySelector('.history')).toBeNull();
  });
});

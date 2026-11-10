import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act } from '../tests/svelte';
import PackSelector from './PackSelector.svelte';

const settings = { priceFloor: 0, priceCap: 0, quantityCap: 0, packWeights: {} };

const packs = [
  { id: 1, name: 'Zendikar Draft Booster', setName: 'Zendikar', productType: 'Draft Booster', quantity: 10, cardsPerPack: 15, marketPrice: 4.99 },
  { id: 2, name: 'Alpha Set Booster', setName: 'Alpha', productType: 'Set Booster', quantity: 10, cardsPerPack: 15, marketPrice: 8.00 },
];

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('PackSelector', () => {
  it('shows empty message when no packs', () => {
    const { container } = render(PackSelector, { props: { packs: [], settings } });
    expect(container.querySelector('.empty')).not.toBeNull();
  });

  it('renders pack list when packs provided', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    expect(container.querySelector('.pack-list')).not.toBeNull();
  });

  it('renders count pill', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    expect(container.querySelector('.count-pill')).not.toBeNull();
  });

  it('renders sort buttons', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const sortBtns = container.querySelectorAll('.sort-btn');
    expect(sortBtns.length).toBeGreaterThanOrEqual(2);
  });

  it('renders Pick a Pack button', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Pick a Pack'),
    );
    expect(btn).not.toBeNull();
  });

  it('Pick a Pack button is enabled when packs are checked', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Pick a Pack'),
    ) as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
  });

  it('renders select all and deselect all buttons', () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const buttons = Array.from(container.querySelectorAll('button')).map((b) => b.textContent);
    expect(buttons.some((t) => t?.includes('Select all'))).toBe(true);
    expect(buttons.some((t) => t?.includes('Deselect all'))).toBe(true);
  });

  it('shows result toast after picking', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      b => b.textContent?.includes('Pick a Pack'),
    ) as HTMLButtonElement;
    await act(() => fireEvent.click(btn));
    expect(container.querySelector('.toast')).not.toBeNull();
    vi.restoreAllMocks();
  });

  it('shows history after picking with recordDraft enabled', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const { container } = render(PackSelector, { props: { packs, settings } });
    const btn = Array.from(container.querySelectorAll('button')).find(
      b => b.textContent?.includes('Pick a Pack'),
    ) as HTMLButtonElement;
    await act(() => fireEvent.click(btn));
    expect(container.querySelector('.history')).not.toBeNull();
    vi.restoreAllMocks();
  });

  it('clicking Price sort activates price sort', async () => {
    const { container } = render(PackSelector, { props: { packs, settings } });
    const sortBtns = container.querySelectorAll('.sort-btn');
    await act(() => fireEvent.click(sortBtns[2])); // Name, Qty, Price
    expect(sortBtns[2].classList.contains('sort-btn--active')).toBe(true);
  });

  describe('pod size & pricing', () => {
    function pickBtn(container: HTMLElement) {
      return Array.from(container.querySelectorAll('button')).find(
        b => b.textContent?.includes('Pick a Pack'),
      ) as HTMLButtonElement;
    }
    const flush = () => new Promise(r => setTimeout(r, 0));

    it('defaults to 4 players / 12 packs', () => {
      const { container } = render(PackSelector, { props: { packs, settings } });
      const input = container.querySelector('#pod-size') as HTMLInputElement;
      expect(input.value).toBe('4');
      expect(container.querySelector('.pod__meta')?.textContent).toContain('12');
    });

    it('changing pod size to 8 means 24 packs and persists', async () => {
      const { container } = render(PackSelector, { props: { packs, settings } });
      const input = container.querySelector('#pod-size') as HTMLInputElement;
      input.value = '8';
      await act(() => fireEvent.change(input));
      expect(container.querySelector('.pod__meta')?.textContent).toContain('24');
      expect(localStorage.getItem('draft-pod-size')).toBe('8');
    });

    it('uses the live price for a pick and shows total and per-player cost', async () => {
      vi.spyOn(Math, 'random').mockReturnValue(0);
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 1, marketPrice: 6.0 }),
      });
      vi.stubGlobal('fetch', fetchMock);
      localStorage.setItem('draft-pod-size', '2');
      const { container } = render(PackSelector, { props: { packs, settings } });
      await act(() => fireEvent.click(pickBtn(container)));
      await act(flush);

      expect(fetchMock).toHaveBeenCalledWith('/api/collection/1/refresh-price', { method: 'POST' });
      const cost = container.querySelector('.cost')?.textContent ?? '';
      expect(cost).toContain('$6.00');  // total
      expect(cost).toContain('$3.00');  // per player (2)
      vi.restoreAllMocks();
    });

    it('multiplies price by packs per slot for small packs', async () => {
      vi.spyOn(Math, 'random').mockReturnValue(0);
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
      const small = [{ id: 9, name: 'Jumpstart', setName: 'Jumpstart', productType: 'Booster', quantity: 9, cardsPerPack: 5, marketPrice: 2.0 }];
      const { container } = render(PackSelector, { props: { packs: small, settings } });
      await act(() => fireEvent.click(pickBtn(container)));
      await act(flush);
      expect(container.querySelector('.history__pack-price')?.textContent).toContain('3× $2.00');
      expect(container.querySelector('.cost')?.textContent).toContain('$6.00');
      vi.restoreAllMocks();
    });

    it('saves the draft with podSize once all picks are made', async () => {
      vi.spyOn(Math, 'random').mockReturnValue(0);
      const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ marketPrice: 5 }) });
      vi.stubGlobal('fetch', fetchMock);
      localStorage.setItem('draft-pod-size', '2');
      const big = [{ ...packs[0], quantity: 50 }];
      const { container } = render(PackSelector, { props: { packs: big, settings } });
      for (let i = 0; i < 6; i++) {
        await act(() => fireEvent.click(pickBtn(container)));
        await act(flush);
      }
      const save = fetchMock.mock.calls.find(c => c[0] === '/api/drafts');
      expect(save).toBeDefined();
      const body = JSON.parse(save![1].body);
      expect(body.podSize).toBe(2);
      expect(body.picks).toHaveLength(6);
      expect(pickBtn(container).disabled).toBe(true);
      vi.restoreAllMocks();
    });

    function rollAllBtn(container: HTMLElement) {
      return Array.from(container.querySelectorAll('button')).find(
        b => b.textContent?.includes('Roll all'),
      ) as HTMLButtonElement;
    }

    it('Roll all fills every slot and saves the draft', async () => {
      const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ marketPrice: 5 }) });
      vi.stubGlobal('fetch', fetchMock);
      localStorage.setItem('draft-pod-size', '8');
      const big = [{ ...packs[0], quantity: 50 }, { ...packs[1], quantity: 50 }];
      const { container } = render(PackSelector, { props: { packs: big, settings } });
      expect(rollAllBtn(container).textContent).toContain('24');

      await act(() => fireEvent.click(rollAllBtn(container)));
      await act(flush);

      // Repeated packs share one line; the per-line counts still add up to 24.
      const items = Array.from(container.querySelectorAll('.history__item'));
      expect(items.length).toBeLessThanOrEqual(2);
      const counts = items.map(li => parseInt(li.querySelector('.history__num')!.textContent!, 10));
      expect(counts.reduce((a, b) => a + b, 0)).toBe(24);
      expect(container.querySelector('.history__title')?.textContent).toContain('24/24');
      const priceCalls = fetchMock.mock.calls.filter(c => String(c[0]).includes('refresh-price'));
      expect(priceCalls.length).toBeLessThanOrEqual(2); // de-duplicated per pack
      const save = fetchMock.mock.calls.find(c => c[0] === '/api/drafts');
      expect(JSON.parse(save![1].body).picks).toHaveLength(24);
      expect(container.querySelector('.cost')?.textContent).toContain('$120.00');
      expect(container.querySelector('.cost')?.textContent).toContain('$15.00');
      expect(rollAllBtn(container).disabled).toBe(true);
    });

    it('Roll all does not start a draft the pool cannot finish', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }));
      const tiny = [{ ...packs[0], quantity: 2 }];
      const { container } = render(PackSelector, { props: { packs: tiny, settings } });
      await act(() => fireEvent.click(rollAllBtn(container)));
      await act(flush);
      expect(container.querySelectorAll('.history__item')).toHaveLength(0);
    });

    it('blocks picking and warns when the pool cannot fill the draft', async () => {
      localStorage.setItem('draft-pod-size', '8'); // 24 slots needed
      const few = [{ ...packs[0], quantity: 5 }, { ...packs[1], quantity: 3 }]; // 8 slots
      const { container } = render(PackSelector, { props: { packs: few, settings } });
      const warning = container.querySelector('.stock-warning');
      expect(warning?.textContent).toContain('8 of the 24');
      expect(pickBtn(container).disabled).toBe(true);
      expect(rollAllBtn(container).disabled).toBe(true);
    });

    it('small packs use several packs per slot when counting stock', async () => {
      localStorage.setItem('draft-pod-size', '2'); // 6 slots needed
      const small = [{ id: 9, name: 'Epilogue', setName: 'Aftermath', productType: 'Epilogue', quantity: 9, cardsPerPack: 5, marketPrice: 7 }]; // 3 slots
      const { container } = render(PackSelector, { props: { packs: small, settings } });
      expect(container.querySelector('.stock-warning')?.textContent).toContain('3 of the 6');
      expect(container.querySelector('.pack-footnote')?.textContent).toContain('3 × $7.00 = $21.00 per slot');
    });

    it('no warning when the pool is large enough', () => {
      const { container } = render(PackSelector, { props: { packs, settings } }); // 20 slots, 12 needed
      expect(container.querySelector('.stock-warning')).toBeNull();
    });
  });
});

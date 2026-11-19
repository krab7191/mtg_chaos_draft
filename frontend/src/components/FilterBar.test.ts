import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, fireEvent, act } from '../tests/svelte';
import FilterBar from './FilterBar.svelte';
import { filters, clearFilters, FILTERS_KEY, setShowOver, withinMax, ALL_LANGUAGES } from '../lib/filters.svelte';

afterEach(() => { clearFilters(); localStorage.clear(); vi.restoreAllMocks(); });

const box = (c: HTMLElement, label: string) =>
  Array.from(c.querySelectorAll('label')).find(l => l.textContent?.trim().startsWith(label))!.querySelector('input')!;

describe('FilterBar', () => {
  it('shares language and max price with every page, remembered in this browser', async () => {
    const { container } = render(FilterBar, {});
    expect(Array.from(container.querySelectorAll('.langs label')).map(l => l.textContent?.trim())[0]).toBe('English');
    expect(container.querySelector('.clear')).toBeNull();

    await act(() => fireEvent.click(box(container, 'French')));
    expect(filters.languages).not.toContain('fr');
    expect(filters.languages).toContain('en');
    expect(JSON.parse(localStorage.getItem(FILTERS_KEY)!).languages).not.toContain('fr');
    expect(container.querySelector('.off')?.textContent?.trim()).toBe('French');

    const max = container.querySelector('.max input') as HTMLInputElement;
    await act(() => fireEvent.change(max, { target: { value: '12' } }));
    expect(filters.maxPrice).toBe(12);
    expect(withinMax(11.99)).toBe(true);
    expect(withinMax(12.5)).toBe(false);
    expect(withinMax(null)).toBe(true);
    await act(() => fireEvent.change(max, { target: { value: 'x' } }));
    expect(filters.maxPrice).toBeNull();
    expect(max.value).toBe('');

    // Ticking French again means "all languages" (no filter); Clear resets everything.
    await act(() => fireEvent.click(box(container, 'French')));
    expect(filters.languages).toBeNull();
    await act(() => fireEvent.click(box(container, 'Korean')));
    await act(() => fireEvent.click(container.querySelector('.clear')!));
    expect(filters.languages).toBeNull();
    expect(container.querySelectorAll('.langs input:checked')).toHaveLength(ALL_LANGUAGES.length);
  });

  it('follows changes made in another tab, and copes without storage', () => {
    localStorage.setItem(FILTERS_KEY, JSON.stringify({ languages: ['ja'], maxPrice: 7 }));
    window.dispatchEvent(new StorageEvent('storage', { key: FILTERS_KEY }));
    expect(filters.languages).toEqual(['ja']);
    expect(filters.maxPrice).toBe(7);
    window.dispatchEvent(new StorageEvent('storage', { key: 'something else' }));
    expect(filters.maxPrice).toBe(7);

    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    window.dispatchEvent(new StorageEvent('storage', { key: FILTERS_KEY }));
    expect(filters.languages).toBeNull(); // unreadable: defaults
    setShowOver(true);
    expect(filters.showOver).toBe(true);
  });
});

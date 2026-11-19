// The viewer's price filters, shared by every prices page (Trends, Cheap
// Packs, Best Deals) and remembered in this browser. One module instance per
// page, so every island on it sees the same state; other tabs follow through
// the storage event.

import { LANGUAGES, NO_FILTERS, type OfferFilters } from './deals';

/** Kept under the key the Cheap Packs filters used, so saved choices carry over. */
export const FILTERS_KEY = 'storeListingFilters';

/** Every language a pack can be listed in, English first. */
export const ALL_LANGUAGES = Object.keys(LANGUAGES);

function load(): OfferFilters {
  try {
    const raw = localStorage.getItem(FILTERS_KEY);
    return raw ? { ...NO_FILTERS, ...JSON.parse(raw) } : { ...NO_FILTERS };
  } catch {
    return { ...NO_FILTERS };
  }
}

export const filters = $state<OfferFilters>(load());

function save() {
  try { localStorage.setItem(FILTERS_KEY, JSON.stringify(filters)); } catch { /* private mode: not remembered */ }
}

/** Shows or hides one language; all ticked again means "no language filter". */
export function setLanguage(lang: string, on: boolean) {
  const current = filters.languages ?? ALL_LANGUAGES;
  const next = on ? [...new Set([...current, lang])] : current.filter(l => l !== lang);
  filters.languages = next.length >= ALL_LANGUAGES.length ? null : ALL_LANGUAGES.filter(l => next.includes(l));
  save();
}

export function setMaxPrice(v: number | null) {
  filters.maxPrice = v != null && Number.isFinite(v) && v > 0 ? v : null;
  save();
}

export function setShowOver(on: boolean) {
  filters.showOver = on;
  save();
}

export function clearFilters() {
  Object.assign(filters, NO_FILTERS);
  save();
}

export const languageShown = (lang: string) => filters.languages == null || filters.languages.includes(lang);

/** True when a price passes the max-price filter. */
export const withinMax = (price: number | null | undefined) => filters.maxPrice == null || price == null || price <= filters.maxPrice;

// Another tab changed the filters: follow it.
if (typeof window !== 'undefined') {
  window.addEventListener('storage', e => {
    if (e.key === FILTERS_KEY) Object.assign(filters, { ...NO_FILTERS }, load());
  });
}

// Store listings of single packs, compared with TCGplayer's market price.

export interface StoreOffer {
  store: string;
  url: string;
  mtgstocksId: number;
  title: string;
  price: number;
  available: boolean;
  /** Pack language: "en", "ja", "de", … ("other" if the store didn't say). */
  language?: string;
  seenAt: string;
  setName: string;
  productType: string;
  releasedAt: string;
  marketPrice: number;
  /** Below market: 0.2 = 20% under; negative = above market. */
  discount: number;
  isDeal: boolean;
  /** The pack's TCGplayer page, where known. */
  tcgUrl?: string;
  /** An English listing 60%+ under market: more likely a mistake than a deal. */
  suspicious?: boolean;
  /** Value of the cards inside (absent if unknown); premium: this listing costs 3×+ that. */
  contents?: number;
  premium?: boolean;
}

/** A pack's value broken down by slot and by card (GET /api/packs/{id}/contents). */
export interface Breakdown {
  mtgstocksId: number;
  setName: string;
  productType: string;
  /** Market price (absent if unknown). */
  price?: number;
  noSales?: boolean;
  /** Expected value of the contents. */
  value: number;
  /** price / value. */
  ratio?: number;
  premium: boolean;
  cards: number;
  slots: { name: string; count: number; value: number }[];
  top: { name: string; foil: boolean; price: number; chance: number; value: number }[];
  pricesDay: string;
}

/** "1 in 12 packs", or "0.8 per pack" when it's likely to show up. */
export function chanceText(chance: number): string {
  if (chance >= 0.5) return `${chance.toFixed(chance >= 2 ? 0 : 1)} per pack`;
  return `1 in ${Math.round(1 / chance)} packs`;
}

/** What a price-to-contents ratio means for buying. */
export function ratioText(ratio: number): string {
  if (ratio >= 3) return `you'd pay ${ratio.toFixed(1)}× what the cards inside are worth: mostly a collectible premium`;
  if (ratio >= 1.2) return `you'd pay ${ratio.toFixed(1)}× the contents (typical for Collector packs: ~1.2–3×)`;
  if (ratio >= 0.8) return `about what the cards inside are worth`;
  return `the cards inside are worth ${(1 / ratio).toFixed(1)}× the price (counting bulk at market, which is hard to sell)`;
}

export interface OffersResponse {
  offers: StoreOffer[];
  /** Stores the app reads daily. */
  stores: string[];
  /** Watched product ids. */
  watched: number[];
  dealDiscount: number;
}

export const LANGUAGES: Record<string, string> = {
  en: 'English', ja: 'Japanese', de: 'German', fr: 'French', it: 'Italian', es: 'Spanish',
  pt: 'Portuguese', zh: 'Chinese', ko: 'Korean', ru: 'Russian', other: 'Foreign',
};

/** A listing's language for display; null for English (the default). */
export function languageLabel(lang: string | undefined): string | null {
  if (!lang || lang === 'en') return null;
  return LANGUAGES[lang] ?? lang.toUpperCase();
}

/** Viewer's filters on the store listings. */
export interface OfferFilters {
  /** Languages to show (codes); null = all. */
  languages: string[] | null;
  /** Highest listing price to show; null = any. */
  maxPrice: number | null;
  /** Also show listings priced above market (hidden by default). */
  showOver: boolean;
}

export const NO_FILTERS: OfferFilters = { languages: null, maxPrice: null, showOver: false };

export function applyFilters(offers: StoreOffer[], f: OfferFilters): StoreOffer[] {
  return offers.filter(o =>
    (f.showOver || o.discount >= 0) &&
    (f.maxPrice == null || o.price <= f.maxPrice) &&
    (f.languages == null || f.languages.includes(o.language ?? 'en')));
}

/** The languages among the listings, English first. */
export function languagesIn(offers: StoreOffer[]): string[] {
  const set = new Set(offers.map(o => o.language ?? 'en'));
  return [...set].sort((a, b) => (a === 'en' ? -1 : b === 'en' ? 1 : (LANGUAGES[a] ?? a).localeCompare(LANGUAGES[b] ?? b)));
}

/** "checked 40 min ago" / "checked 3 h ago" for a listing's last read. */
export function checkedAgo(iso: string, now = new Date()): string {
  const min = Math.max(0, Math.round((now.getTime() - new Date(iso).getTime()) / 60_000));
  if (!Number.isFinite(min)) return '';
  if (min < 60) return `checked ${min} min ago`;
  if (min < 48 * 60) return `checked ${Math.round(min / 60)} h ago`;
  return `checked ${Math.round(min / 1440)} days ago`;
}

/** One pack's listings: the best one (first in the given order) and the rest. */
export interface OfferGroup { best: StoreOffer; others: StoreOffer[] }

/** Groups listings by pack, keeping the given order (best discount first). */
export function groupByPack(offers: StoreOffer[]): OfferGroup[] {
  const groups = new Map<number, OfferGroup>();
  for (const o of offers) {
    const g = groups.get(o.mtgstocksId);
    if (g) g.others.push(o);
    else groups.set(o.mtgstocksId, { best: o, others: [] });
  }
  return [...groups.values()];
}

export type OfferView = 'deals' | 'instock' | 'all';

export function filterOffers(offers: StoreOffer[], view: OfferView): StoreOffer[] {
  if (view === 'deals') return offers.filter(o => o.isDeal);
  if (view === 'instock') return offers.filter(o => o.available);
  return offers;
}

/** What to search other stores for: "Prophecy Booster Pack", "Kaldheim Set Booster". */
export function searchQuery(setName: string, productType: string): string {
  return productType === 'Booster Pack' ? `${setName} Booster Pack` : `${setName} ${productType}`;
}

// Dave & Adam's set names where they differ from MTGStocks'.
const DA_SETS: Record<string, string> = {
  'Alpha Edition': 'Alpha',
  'Ikoria: Lair of Behemoths': 'Ikoria Lair of Behemoths',
  'Kamigawa: Neon Dynasty': 'Kamigawa Neon Dynasty',
  'Theros: Beyond Death': 'Theros Beyond Death',
  'The Lord of the Rings: Tales of Middle-earth': 'The Lord of the Rings',
  "Marvel's Spider-Man": 'Spider-Man',
};

/** A set's name in Dave & Adam's set filter. */
export function daveAndAdamsSet(setName: string): string {
  if (DA_SETS[setName]) return DA_SETS[setName];
  return setName
    .replace(/^Core Set (\d{4})$/, 'Magic $1') // "Core Set 2020" → "Magic 2020"
    .replace(/ \(M\d+\)$/, '')               // "Magic 2010 (M10)" → "Magic 2010"
    .replace(/^Universes Beyond: /, '');      // "Universes Beyond: Final Fantasy" → "Final Fantasy"
}

/** Lowercase, apostrophes dropped, anything else non-alphanumeric as one hyphen. */
function slug(s: string): string {
  return s.toLowerCase().replace(/['’]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

/** Cardmarket's booster page name: "Dragon's Maze" → "Dragon-s-Maze-Booster",
 *  "Bloomburrow" Play Booster → "Bloomburrow-Play-Booster". */
export function cardmarketSlug(setName: string, productType: string): string {
  const kind = productType === 'Booster Pack' ? 'Booster' : productType;
  return `${setName} ${kind}`.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

/** Stores the app can't read (no feed, or they block automated access):
 *  a one-click search each, opened by hand. */
/** A store to look at by hand; note: a caveat worth knowing before buying (shown as a tooltip and tag). */
export interface SearchLink { store: string; url: string; note?: string }

export function searchLinks(setName: string, productType: string, tcgUrl?: string): SearchLink[] {
  const q = encodeURIComponent(searchQuery(setName, productType));
  return [
    // The pack's own page when known; a search otherwise (it also lists prerelease kits and such).
    { store: 'TCGplayer', url: tcgUrl ? `${tcgUrl}?Language=English` : `https://www.tcgplayer.com/search/magic/product?q=${q}&productLineName=magic` },
    { store: 'Card Kingdom', url: `https://www.cardkingdom.com/catalog/search?search=header&filter%5Bname%5D=${q}` },
    { store: 'eBay', url: `https://www.ebay.com/sch/i.html?_nkw=${q}&LH_BIN=1&LH_ItemCondition=1000` },
    { store: 'Troll and Toad', url: `https://www.trollandtoad.com/category.php?search-words=${q}` },
    // Their search address 404s; their Magic boxes-and-packs list is short (~80 items).
    { store: 'Miniature Market (all MTG packs)', url: 'https://www.miniaturemarket.com/trading-card-games/magic-the-gathering.html?limit=100&order=best_seller&p=1&properties=019959097ce27034829069c675c1b6f8%7C01995909920d74bb87b962a425474041' },
    // Their site search ignores the query; their boosters page filters by set.
    { store: "Dave & Adam's", url: `https://www.dacardworld.com/gaming/booster-boxes-from-the-magic-the-gathering-tcg?set=${encodeURIComponent(daveAndAdamsSet(setName))}` },
    // European (prices in euros); their search mixes in other games, so open their Magic section.
    { store: 'Magic Omens (EU, all MTG)', url: 'https://magicomens.com/collections/mtg', note: 'Ships from Europe: only worth it for a bigger order' },
    { store: 'Galactic Toys', url: `https://galactictoys.com/search?q=${q}`, note: 'Shipping is $8+ per order: only worth it for several packs' },
    { store: 'Collector Store', url: `https://www.collectorstore.com/search?q=${q}` },
    // A sealed page per set: "Dragon's Maze" → /mtg/dragons-maze/sealed/.
    { store: 'Star City Games', url: `https://starcitygames.com/mtg/${slug(setName)}/sealed/` },
    // Their packs section; no language filter, so foreign packs show too.
    { store: 'ABU Games', url: `https://abugames.com/magic-the-gathering/packs?search=${q}` },
    { store: 'CoolStuffInc', url: `https://www.coolstuffinc.com/main_search.php?pa=searchOnName&page=1&resultsPerPage=25&q=${q}` },
    { store: 'Hareruya', url: `https://www.hareruyamtg.com/en/products/search?product=${q}`, note: 'Ships from Japan: only worth it for a bigger order' },
    { store: 'Cardmarket (EU)', url: `https://www.cardmarket.com/en/Magic/Products/Boosters/${cardmarketSlug(setName, productType)}`, note: 'Ships from Europe: only worth it for a bigger order' },
  ];
}

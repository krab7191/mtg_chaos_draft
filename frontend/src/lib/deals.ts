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
}

export interface OffersResponse {
  offers: StoreOffer[];
  /** Stores the app reads daily. */
  stores: string[];
  /** Watched product ids. */
  watched: number[];
  dealDiscount: number;
}

const LANGUAGES: Record<string, string> = {
  en: 'English', ja: 'Japanese', de: 'German', fr: 'French', it: 'Italian', es: 'Spanish',
  pt: 'Portuguese', zh: 'Chinese', ko: 'Korean', ru: 'Russian', other: 'Foreign',
};

/** A listing's language for display; null for English (the default). */
export function languageLabel(lang: string | undefined): string | null {
  if (!lang || lang === 'en') return null;
  return LANGUAGES[lang] ?? lang.toUpperCase();
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
export function searchLinks(setName: string, productType: string, tcgUrl?: string): { store: string; url: string }[] {
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
    { store: 'Magic Omens (EU, all MTG)', url: 'https://magicomens.com/collections/mtg' },
    { store: 'Collector Store', url: `https://www.collectorstore.com/search?q=${q}` },
    // A sealed page per set: "Dragon's Maze" → /mtg/dragons-maze/sealed/.
    { store: 'Star City Games', url: `https://starcitygames.com/mtg/${slug(setName)}/sealed/` },
    { store: 'ABU Games', url: `https://abugames.com/magic-the-gathering/sealed?search=${q}` },
    { store: 'CoolStuffInc', url: `https://www.coolstuffinc.com/main_search.php?pa=searchOnName&page=1&resultsPerPage=25&q=${q}` },
    { store: 'Gamenerdz', url: `https://www.gamenerdz.com/search.php?search_query=${q}` },
    { store: 'Mercari', url: `https://www.mercari.com/search/?keyword=${q}` },
    { store: 'Hareruya', url: `https://www.hareruyamtg.com/en/products/search?product=${q}` },
    { store: 'Cardmarket (EU)', url: `https://www.cardmarket.com/en/Magic/Products/Boosters/${cardmarketSlug(setName, productType)}` },
  ];
}

import { describe, it, expect } from 'vitest';
import { filterOffers, applyFilters, languagesIn, groupByPack, NO_FILTERS, searchQuery, searchLinks, daveAndAdamsSet, cardmarketSlug, languageLabel, chanceText, ratioText, type StoreOffer } from './deals';

const offer = (over: Partial<StoreOffer> = {}): StoreOffer => ({
  store: 'Collector Store', url: 'https://x/p', mtgstocksId: 1, title: 'Prophecy Booster Pack', price: 40, available: true,
  seenAt: '2026-11-19T00:00:00Z', setName: 'Prophecy', productType: 'Booster Pack', releasedAt: '2000-06-05',
  marketPrice: 56, discount: 0.29, isDeal: true, ...over,
});

describe('deals', () => {
  it('filters to deals, in stock, or everything', () => {
    const all = [offer(), offer({ url: 'b', isDeal: false }), offer({ url: 'c', isDeal: false, available: false })];
    expect(filterOffers(all, 'deals').map(o => o.url)).toEqual(['https://x/p']);
    expect(filterOffers(all, 'instock').map(o => o.url)).toEqual(['https://x/p', 'b']);
    expect(filterOffers(all, 'all')).toHaveLength(3);
  });

  it('searches other stores for the pack by name', () => {
    expect(searchQuery('Prophecy', 'Booster Pack')).toBe('Prophecy Booster Pack');
    expect(searchQuery('Kaldheim', 'Set Booster')).toBe('Kaldheim Set Booster');
    const links = searchLinks('Shards of Alara', 'Booster Pack');
    expect(links.map(l => l.store)).toContain('eBay');
    expect(links.every(l => l.url.startsWith('https://'))).toBe(true);
    // Every store searches for the pack, except those whose search doesn't work: Dave & Adam's
    // (filtered by set), Miniature Market and Magic Omens (their Magic sections).
    expect(links.filter(l => !l.url.includes('Shards%20of%20Alara%20Booster%20Pack')).map(l => l.store))
      .toEqual(['Miniature Market (all MTG packs)', "Dave & Adam's", 'Magic Omens (EU, all MTG)', 'Star City Games', 'Cardmarket (EU)']);
    expect(searchLinks("Dragon's Maze", 'Booster Pack').find(l => l.store === 'Star City Games')!.url)
      .toBe('https://starcitygames.com/mtg/dragons-maze/sealed/');
    expect(searchLinks('Fallen Empires', 'Booster Pack').find(l => l.store === 'ABU Games')!.url)
      .toBe('https://abugames.com/magic-the-gathering/packs?search=Fallen%20Empires%20Booster%20Pack');
    expect(searchLinks("Dragon's Maze", 'Booster Pack').find(l => l.store === 'Cardmarket (EU)')!.url)
      .toBe('https://www.cardmarket.com/en/Magic/Products/Boosters/Dragon-s-Maze-Booster'); // the user's real link
    expect(links.find(l => l.store === "Dave & Adam's")!.url)
      .toBe('https://www.dacardworld.com/gaming/booster-boxes-from-the-magic-the-gathering-tcg?set=Shards%20of%20Alara');
    // eBay: new condition, Buy It Now only.
    expect(links.find(l => l.store === 'eBay')!.url).toContain('LH_BIN=1');
  });

  it("links TCGplayer's product page when known", () => {
    const tcg = (u?: string) => searchLinks("Dragon's Maze", 'Booster Pack', u).find(l => l.store === 'TCGplayer')!.url;
    expect(tcg('https://www.tcgplayer.com/product/67752/magic-dragons-maze-dragons-maze-booster-pack'))
      .toBe('https://www.tcgplayer.com/product/67752/magic-dragons-maze-dragons-maze-booster-pack?Language=English');
    expect(tcg()).toContain('/search/magic/product?q=Dragon');
  });

  it('words pull chances and price-to-contents ratios', () => {
    expect(chanceText(1 / 55)).toBe('1 in 55 packs');
    expect(chanceText(0.75)).toBe('0.8 per pack');
    expect(chanceText(5)).toBe('5 per pack');
    expect(ratioText(13.7)).toContain('13.7× what the cards inside are worth');
    expect(ratioText(2)).toContain('typical for Collector packs');
    expect(ratioText(1)).toBe('about what the cards inside are worth');
    expect(ratioText(0.5)).toContain('worth 2.0× the price');
  });

  it('filters by language, max price and above-market, and lists languages English first', () => {
    const all = [offer(), offer({ url: 'b', language: 'it', price: 30, discount: -0.2 }), offer({ url: 'c', language: 'ja', price: 8 })];
    expect(applyFilters(all, NO_FILTERS).map(o => o.url)).toEqual(['https://x/p', 'c']);
    expect(applyFilters(all, { ...NO_FILTERS, showOver: true }).map(o => o.url)).toEqual(['https://x/p', 'b', 'c']);
    expect(applyFilters(all, { ...NO_FILTERS, languages: ['ja'] }).map(o => o.url)).toEqual(['c']);
    expect(applyFilters(all, { ...NO_FILTERS, maxPrice: 10 }).map(o => o.url)).toEqual(['c']);
    expect(languagesIn([offer({ language: 'ja' }), offer({ language: 'de' }), offer({ language: undefined })])).toEqual(['en', 'de', 'ja']);
  });

  it('groups listings by pack, best first', () => {
    const g = groupByPack([offer({ url: 'a' }), offer({ url: 'b', mtgstocksId: 2 }), offer({ url: 'c' })]);
    expect(g.map(x => [x.best.url, x.others.map(o => o.url)])).toEqual([['a', ['c']], ['b', []]]);
  });

  it('labels foreign-language packs', () => {
    expect(languageLabel(undefined)).toBeNull();
    expect(languageLabel('en')).toBeNull();
    expect(languageLabel('ja')).toBe('Japanese');
    expect(languageLabel('other')).toBe('Foreign');
    expect(languageLabel('xx')).toBe('XX');
  });

  it('builds Cardmarket booster page names', () => {
    expect(cardmarketSlug("Dragon's Maze", 'Booster Pack')).toBe('Dragon-s-Maze-Booster');
    expect(cardmarketSlug('Bloomburrow', 'Play Booster')).toBe('Bloomburrow-Play-Booster');
    expect(cardmarketSlug('Duskmourn: House of Horror', 'Collector Booster')).toBe('Duskmourn-House-of-Horror-Collector-Booster');
  });

  it("uses Dave & Adam's spelling of set names", () => {
    expect(daveAndAdamsSet('Ikoria: Lair of Behemoths')).toBe('Ikoria Lair of Behemoths');
    expect(daveAndAdamsSet('Core Set 2020')).toBe('Magic 2020');
    expect(daveAndAdamsSet('Magic 2014 (M14)')).toBe('Magic 2014');
    expect(daveAndAdamsSet('Universes Beyond: Final Fantasy')).toBe('Final Fantasy');
    expect(daveAndAdamsSet('The Lord of the Rings: Tales of Middle-earth')).toBe('The Lord of the Rings');
    expect(daveAndAdamsSet('Innistrad: Crimson Vow')).toBe('Innistrad: Crimson Vow'); // same name: unchanged
  });
});

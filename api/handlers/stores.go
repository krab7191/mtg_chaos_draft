package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store listings: online stores' single booster packs, read from feeds the
// stores publish (Shopify's public product JSON), matched to MTGStocks
// products, and compared with TCGplayer's market price to spot deals.
//
// Stores rate-limit quick clients hard (Collector Store refuses a second
// page even 30s after the first), so a store is read one page at a time,
// storePageDelay apart, once a day in the background. A refused page is
// retried once after storeRetryDelay; any other failure stops that store's
// run and keeps its last complete listing until the next day.

// shopifyStore is a store on Shopify and the collection holding its Magic products.
type shopifyStore struct {
	Name       string
	Domain     string // e.g. www.collectorstore.com
	Collection string // e.g. games-magic-the-gathering
}

var shopifyStores = []shopifyStore{
	{Name: "Collector Store", Domain: "www.collectorstore.com", Collection: "games-magic-the-gathering"},
	{Name: "Galactic Toys", Domain: "galactictoys.com", Collection: "magic-sealed-booster-packs"},
}

var (
	storePageDelay  = 2 * time.Minute
	storeRetryDelay = 10 * time.Minute
	storeMaxPages   = 20
	// Store requests go through MTGSTOCKS_PROXY (the home connection over
	// WireGuard) when set, like MTGStocks: some stores refuse data-center
	// addresses. If the tunnel is down they fail fast and keep the last listing.
	storeClient = newProxiedClient(os.Getenv("MTGSTOCKS_PROXY"), 60*time.Second)
	// storeBaseURL overrides https://<domain> in tests.
	storeBaseURL = func(domain string) string { return "https://" + domain }
)

// dealDiscount: a listing this far under TCGplayer's market price is a deal.
const dealDiscount = 0.15

// ── Shopify feed ──────────────────────────────────────────────

type shopifyProduct struct {
	Title    string `json:"title"`
	Handle   string `json:"handle"`
	Variants []struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Price     string `json:"price"`
		Available bool   `json:"available"`
	} `json:"variants"`
}

// fetchShopify reads a store's whole collection, page by page.
func fetchShopify(ctx context.Context, s shopifyStore) ([]shopifyProduct, error) {
	var all []shopifyProduct
	for page := 1; page <= storeMaxPages; page++ {
		if page > 1 {
			select {
			case <-time.After(storePageDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		url := fmt.Sprintf("%s/collections/%s/products.json?limit=250&page=%d", storeBaseURL(s.Domain), s.Collection, page)
		body, status, err := getStorePage(ctx, url)
		if err == nil && status == http.StatusTooManyRequests {
			select { // refused: wait a good while, then one more try
			case <-time.After(storeRetryDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			body, status, err = getStorePage(ctx, url)
		}
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%s page %d: status %d", s.Name, page, status)
		}
		var data struct {
			Products []shopifyProduct `json:"products"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("%s page %d: %w", s.Name, page, err)
		}
		all = append(all, data.Products...)
		if len(data.Products) < 250 {
			return all, nil
		}
	}
	return all, nil
}

func getStorePage(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; pack-price-check)")
	req.Header.Set("Accept", "application/json")
	resp, err := storeClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return body, resp.StatusCode, err
}

// shopifyOffers turns a store's products into pack listings it can match.
func shopifyOffers(s shopifyStore, products []shopifyProduct, idx *packIndex) []db.Offer {
	var out []db.Offer
	for _, p := range products {
		for _, v := range p.Variants {
			title := p.Title
			multi := len(p.Variants) > 1
			if multi {
				title += " - " + v.Title
			}
			pack, ok := idx.match(title)
			if !ok {
				continue
			}
			price, err := strconv.ParseFloat(v.Price, 64)
			if err != nil || price <= 0 {
				continue
			}
			url := storeBaseURL(s.Domain) + "/products/" + p.Handle
			if multi {
				url += fmt.Sprintf("?variant=%d", v.ID)
			}
			out = append(out, db.Offer{Store: s.Name, URL: url, MTGStocksID: pack.MTGStocksID, Title: title, Price: price, Available: v.Available, Language: languageOf(title)})
		}
	}
	return out
}

// ── Matching titles to packs ──────────────────────────────────

// languages by the words stores use for them; English when none appears.
// Foreign packs are fine for chaos drafts, so they're kept and labelled.
var languageWords = map[string]string{
	"japanese": "ja", "jpn": "ja", "jp": "ja",
	"german": "de", "deutsch": "de",
	"french": "fr", "francais": "fr",
	"italian": "it", "italiano": "it",
	"spanish": "es", "espanol": "es",
	"portuguese": "pt",
	"chinese":    "zh", "simplified": "zh", "traditional": "zh",
	"korean":  "ko",
	"russian": "ru",
	"foreign": "other",
}

// languageOf returns a listing title's pack language ("en" by default).
func languageOf(title string) string {
	for _, w := range strings.Fields(normTitle(title)) {
		if lang, ok := languageWords[w]; ok {
			return lang
		}
	}
	return "en"
}

// packWords may appear in a single pack's title besides the set name.
var packWords = map[string]bool{
	"booster": true, "pack": true, "draft": true, "set": true, "play": true, "collector": true, "collectors": true,
	"single": true, "sealed": true, "factory": true, "new": true, "english": true, "non": true, "edition": true,
	"1": true, "1pack": true, "one": true, "default": true, "title": true, "magic": true, "the": true, "gathering": true,
	"mtg": true, "and": true, "of": true,
	// Real pack names seen at stores: "3rd Edition Revised", Mirrodin Besieged's
	// Mirran/Phyrexian faction packs, "Ravnica: City of Guilds".
	"3rd": true, "faction": true, "mirran": true, "phyrexian": true, "city": true, "guilds": true,
}

// notAPack: listings that aren't one sealed booster pack.
var notAPack = regexp.MustCompile(`\b(box|boxes|display|case|bundle|blister|lot|lots|theme|starter|tournament|prerelease|fat|gift|deck|decks|empty|opened|repack|repacked|resealed|wrapper|proxy|art|sample|jumpstart|commander|omega|collection|mystery|sleeve|sleeves|premium|foil|vip|anniversary|demo|intro|welcome|arena|sampler|player|promo|promos|tutorial|trial|kit|kits|convention)\b|\bspecial edition\b|\b(six|6) card\b|\bset of\b|\bpack of\b|\b\d+\s*x\b|\bx\s*\d+\b|\b\d+\s*(packs|boosters)\b|\(\d+\)`)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normTitle lowercases, spells "&" as "and", drops apostrophes and turns other
// punctuation into spaces, padded so " word " matches whole words.
func normTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.NewReplacer("'", "", "’", "").Replace(s)
	return " " + strings.TrimSpace(nonAlnum.ReplaceAllString(s, " ")) + " "
}

// parenthetical matches a trailing " m10 " style code left from "(M10)".
var parenthetical = regexp.MustCompile(` m\d+ $`)

// coreSetYear: "2010 Core Set" is MTGStocks' "Magic 2010".
var coreSetYear = regexp.MustCompile(` (20\d\d) core set `)

// setKey normalizes a set name the same way titles are normalized.
func setKey(name string) string { return ordinals.Replace(normTitle(name)) }

// ordinals: spell "4th Edition" and "Fourth Edition" alike, on both sides.
var ordinals = strings.NewReplacer(" 4th ", " fourth ", " 5th ", " fifth ", " 6th ", " sixth ", " 7th ", " seventh ", " 8th ", " eighth ", " 9th ", " ninth ", " 10th ", " tenth ")

type setPacks struct {
	keys   []string                // normalized names: "shards of alara", ...
	byKind map[string]SearchResult // "booster", "draft", "set", "play", "collector"
}

// packIndex finds the catalog pack a store title names.
type packIndex struct {
	sets []*setPacks
}

func packKindOf(productType string) string {
	switch productType {
	case "Booster Pack":
		return "booster"
	case "Draft Booster":
		return "draft"
	case "Set Booster":
		return "set"
	case "Play Booster":
		return "play"
	case "Collector Booster":
		return "collector"
	}
	return ""
}

func newPackIndex(products []SearchResult) *packIndex {
	bySet := map[string]*setPacks{}
	for _, p := range products {
		kind := packKindOf(p.ProductType)
		if kind == "" || isJumpstart(p.SetName, p.ProductType) {
			continue
		}
		sp := bySet[p.SetName]
		if sp == nil {
			name := setKey(p.SetName)
			sp = &setPacks{keys: []string{name}, byKind: map[string]SearchResult{}}
			for _, alt := range []string{
				strings.TrimSuffix(name, " edition ") + " ",         // "Revised Edition" → "Revised"
				strings.Replace(name, " classic ", " ", 1),          // "Classic Sixth Edition" → "Sixth Edition"
				strings.Replace(name, " universes beyond ", " ", 1), // "Universes Beyond: Fallout" → "Fallout"
				parenthetical.ReplaceAllString(name, " "),           // "Magic 2010 (M10)" → "Magic 2010"
			} {
				if alt != name && strings.TrimSpace(alt) != "" && !slices.Contains(sp.keys, alt) {
					sp.keys = append(sp.keys, alt)
				}
			}
			bySet[p.SetName] = sp
		}
		sp.byKind[kind] = p
	}
	// Stores often drop a subtitle: "Duskmourn" for "Duskmourn: House of
	// Horror". Use the part before the colon too, unless another set has
	// that name or the same short name ("Theros" vs "Theros: Beyond Death").
	full, short := map[string]bool{}, map[string][]*setPacks{}
	for name, sp := range bySet {
		full[sp.keys[0]] = true
		if i := strings.Index(name, ":"); i > 0 {
			k := setKey(name[:i])
			short[k] = append(short[k], sp)
		}
	}
	for k, sps := range short {
		if len(sps) == 1 && !full[k] {
			sps[0].keys = append(sps[0].keys, k)
		}
	}
	idx := &packIndex{}
	for _, sp := range bySet {
		idx.sets = append(idx.sets, sp)
	}
	// Fixed order, so ties never depend on map order.
	sort.Slice(idx.sets, func(i, j int) bool { return idx.sets[i].keys[0] < idx.sets[j].keys[0] })
	return idx
}

// match returns the pack a listing title names, if it clearly names one
// single English booster pack of a known set.
func (idx *packIndex) match(title string) (SearchResult, bool) {
	t := coreSetYear.ReplaceAllString(setKey(title), " magic $1 ")
	t = strings.ReplaceAll(t, " spiderman ", " spider man ") // "Marvel's Spiderman"
	if notAPack.MatchString(t) || !strings.Contains(t, " booster ") && !strings.Contains(t, " pack ") {
		return SearchResult{}, false
	}
	t = strings.NewReplacer(" magic the gathering ", " ", " mtg ", " ").Replace(t)

	// The longest set name in the title wins: "Alara Reborn" over "Alara".
	var best *setPacks
	bestKey := ""
	for _, sp := range idx.sets {
		for _, k := range sp.keys {
			if len(k) > len(bestKey) && strings.Contains(t, k) {
				best, bestKey = sp, k
			}
		}
	}
	if best == nil {
		return SearchResult{}, false
	}
	rest := strings.Replace(t, bestKey, " ", 1) // what's left says the pack kind

	has := func(w string) bool { return strings.Contains(rest, " "+w+" ") }
	var kind string
	switch {
	case has("collector") || has("collectors"):
		kind = "collector"
	case has("set") && has("booster"):
		kind = "set"
	case has("play"):
		kind = "play"
	case has("draft"):
		kind = "draft"
	default:
		// A plain "Booster Pack": the set's only ordinary booster.
		var plain []string
		for _, k := range []string{"booster", "draft", "play"} {
			if _, ok := best.byKind[k]; ok {
				plain = append(plain, k)
			}
		}
		if len(plain) != 1 {
			return SearchResult{}, false
		}
		if _, hasSet := best.byKind["set"]; hasSet && plain[0] == "draft" {
			return SearchResult{}, false // draft or set booster? can't tell
		}
		kind = plain[0]
	}
	// Whatever else the title says must be ordinary pack words: an unexpected
	// word ("Demo Game", "Sampler", "Two Player") means some other product.
	for _, w := range strings.Fields(rest) {
		if !packWords[w] && languageWords[w] == "" {
			return SearchResult{}, false
		}
	}
	p, ok := best.byKind[kind]
	if !ok && kind == "draft" {
		// Older sets' draft boosters are plain "Booster Pack"s in MTGStocks.
		p, ok = best.byKind["booster"]
	}
	return p, ok
}

// ── Card Kingdom ──────────────────────────────────────────────

// Card Kingdom publishes its whole sealed catalog (price, stock, page) as one
// JSON file, so it's a single request a day.
const cardKingdomName = "Card Kingdom"

var cardKingdomURL = "https://api.cardkingdom.com/api/sealed_pricelist"

type ckItem struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	PriceRetail string `json:"price_retail"`
	QtyRetail   int    `json:"qty_retail"`
}

func fetchCardKingdom(ctx context.Context, idx *packIndex) ([]db.Offer, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardKingdomURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; pack-price-check)")
	resp, err := storeClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%s: status %d", cardKingdomName, resp.StatusCode)
	}
	var data struct {
		Meta struct {
			BaseURL string `json:"base_url"`
		} `json:"meta"`
		Data []ckItem `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&data); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", cardKingdomName, err)
	}
	base := data.Meta.BaseURL
	if base == "" {
		base = "https://www.cardkingdom.com/"
	}
	var out []db.Offer
	for _, it := range data.Data {
		pack, ok := idx.match(it.Name)
		if !ok {
			continue
		}
		price, err := strconv.ParseFloat(it.PriceRetail, 64)
		if err != nil || price <= 0 {
			continue
		}
		out = append(out, db.Offer{Store: cardKingdomName, URL: base + it.URL, MTGStocksID: pack.MTGStocksID,
			Title: it.Name, Price: price, Available: it.QtyRetail > 0, Language: languageOf(it.Name)})
	}
	return out, len(data.Data), nil
}

// ── Daily refresh ─────────────────────────────────────────────

// RefreshStoreOffers re-reads every store once. A store that fails keeps its
// last complete listing.
func RefreshStoreOffers(ctx context.Context, pool *pgxpool.Pool) {
	c, err := getCatalog()
	if err != nil {
		log.Printf("store listings: no catalog: %v", err)
		return
	}
	idx := newPackIndex(c.products)
	save := func(store string, offers []db.Offer, from int) {
		if err := db.ReplaceStoreOffers(ctx, pool, store, offers); err != nil {
			log.Printf("store listings: saving %s: %v", store, err)
			return
		}
		log.Printf("store listings: %s: %d packs matched from %d products", store, len(offers), from)
	}
	for _, s := range shopifyStores {
		products, err := fetchShopify(ctx, s)
		if err != nil {
			log.Printf("store listings: %v (keeping the last listing)", err)
			continue
		}
		save(s.Name, shopifyOffers(s, products, idx), len(products))
	}
	if cardKingdomURL != "" {
		if offers, n, err := fetchCardKingdom(ctx, idx); err != nil {
			log.Printf("store listings: %v (keeping the last listing)", err)
		} else {
			save(cardKingdomName, offers, n)
		}
	}
	if cardTraderToken != "" {
		if offers, n, err := fetchCardTrader(ctx, pool, c, idx); err != nil {
			log.Printf("store listings: %v (keeping the last listing)", err)
		} else {
			save(cardTraderName, offers, n)
		}
	}
}

// ── API ───────────────────────────────────────────────────────

// Deal is a store listing with how it compares to the market price.
type Deal struct {
	db.Offer
	SetName     string  `json:"setName"`
	ProductType string  `json:"productType"`
	ReleasedAt  string  `json:"releasedAt"`
	MarketPrice float64 `json:"marketPrice"`
	// Discount below market: 0.2 = 20% under; negative = above market.
	Discount float64 `json:"discount"`
	IsDeal   bool    `json:"isDeal"`
	// Preorder: the product isn't released yet. Its "market" price is
	// preorder hype, so a listing under it is never called a deal.
	Preorder bool `json:"preorder,omitempty"`
	// TCGURL is the pack's TCGplayer page, where known.
	TCGURL string `json:"tcgUrl,omitempty"`
	// Contents is the value of the cards inside (0 if unknown); Premium
	// means this listing costs premiumRatio× that or more.
	Contents float64 `json:"contents,omitempty"`
	Premium  bool    `json:"premium,omitempty"`
}

// offersWithMarket joins listings with market prices, best discount first.
func offersWithMarket(offers []db.Offer, products []SearchResult) []Deal {
	today := time.Now().UTC().Format(time.DateOnly)
	byID := map[int]SearchResult{}
	for _, p := range products {
		byID[p.MTGStocksID] = p
	}
	out := []Deal{}
	for _, o := range offers {
		p, ok := byID[o.MTGStocksID]
		// No market price, or only asking prices (no recent sales): there's
		// nothing real to compare with, so the listing isn't shown.
		if !ok || p.MarketPrice == nil || *p.MarketPrice <= 0 || p.NoSales {
			continue
		}
		d := Deal{Offer: o, SetName: p.SetName, ProductType: p.ProductType, ReleasedAt: p.ReleasedAt, MarketPrice: *p.MarketPrice, TCGURL: tcgLink(o.MTGStocksID)}
		d.Discount = 1 - o.Price / *p.MarketPrice
		d.Preorder = p.ReleasedAt > today
		if v := packValue(o.MTGStocksID); v > 0 {
			d.Contents = v
			d.Premium = o.Price >= premiumRatio*v
		}
		d.IsDeal = o.Available && !d.Preorder && d.Discount >= dealDiscount
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Discount > out[j].Discount })
	return out
}

// bestDeals is each product's cheapest in-stock listing that counts as a deal.
func bestDeals(offers []db.Offer, products []SearchResult) map[int]*Deal {
	out := map[int]*Deal{}
	all := offersWithMarket(offers, products)
	for i := range all {
		d := &all[i]
		if b, ok := out[d.MTGStocksID]; d.IsDeal && (!ok || d.Price < b.Price) {
			out[d.MTGStocksID] = d
		}
	}
	return out
}

// StoreOffers lists store listings of single packs with their discount.
func (h *TrendsHandler) StoreOffers(w http.ResponseWriter, r *http.Request) {
	c, err := getCatalog()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	offers, err := db.ListStoreOffers(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	watches, err := db.ListWatches(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	resp := struct {
		Offers       []Deal   `json:"offers"`
		Stores       []string `json:"stores"`
		Watched      []int    `json:"watched"`
		DealDiscount float64  `json:"dealDiscount"`
	}{Offers: offersWithMarket(offers, c.products), Stores: []string{}, Watched: []int{}, DealDiscount: dealDiscount}
	for id := range watches {
		resp.Watched = append(resp.Watched, id)
	}
	sort.Ints(resp.Watched)
	for _, s := range shopifyStores {
		resp.Stores = append(resp.Stores, s.Name)
	}
	if cardKingdomURL != "" {
		resp.Stores = append(resp.Stores, cardKingdomName)
	}
	if cardTraderToken != "" {
		resp.Stores = append(resp.Stores, cardTraderName)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

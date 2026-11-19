package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestPackIndex_MatchesRealTitles(t *testing.T) {
	c, _ := loadRealCatalog(t)
	idx := newPackIndex(c.products)
	want := map[string]string{ // title -> "Set | Type"
		"Magic the Gathering: Prophecy Booster Pack":            "Prophecy | Booster Pack",
		"Magic the Gathering: Judgment Booster Pack":            "Judgment | Booster Pack",
		"Magic the Gathering: Shards of Alara Booster Pack":     "Shards of Alara | Booster Pack",
		"MTG Alara Reborn Booster Pack":                         "Alara Reborn | Booster Pack",
		"Magic: The Gathering - 4th Edition Booster Pack":       "Fourth Edition | Booster Pack",
		"Revised Booster Pack":                                  "Revised Edition | Booster Pack",
		"Magic the Gathering: Duskmourn Play Booster Pack":      "Duskmourn: House of Horror | Play Booster",
		"Duskmourn: House of Horror Collector Booster":          "Duskmourn: House of Horror | Collector Booster",
		"Kaldheim Set Booster Pack":                             "Kaldheim | Set Booster",
		"Kaldheim Draft Booster Pack":                           "Kaldheim | Draft Booster",
		"Magic the Gathering: Mirage Booster Pack":              "Mirage | Booster Pack",
		"Magic the Gathering: Theros Beyond Death Booster":      "Theros: Beyond Death | Booster Pack",
		"Theros Booster Pack":                                   "Theros | Booster Pack",
		"Zendikar Rising Booster Pack":                          "", // draft or set booster? can't tell
		"Magic the Gathering: Scourge Booster Box":              "", // a box
		"Magic the Gathering: Prophecy Booster Pack (Japanese)": "",
		"Magic the Gathering: Onslaught Booster Pack Lot (3)":   "",
		"Magic the Gathering: Mirage Booster Pack x2":           "",
		"Magic the Gathering: Tempest Theme Deck":               "",
		"Magic the Gathering: Kaldheim Jumpstart Booster Pack":  "",
		"Pokemon Scarlet & Violet Booster Pack":                 "",
	}
	for title, exp := range want {
		p, ok := idx.match(title)
		got := ""
		if ok {
			got = p.SetName + " | " + p.ProductType
		}
		if got != exp {
			t.Errorf("%q: got %q, want %q", title, got, exp)
		}
	}
}

// Tests never reach real stores: individual tests point a store at a fake.
func init() { shopifyStores, cardKingdomURL = nil, "" }

// fakeShopify serves a collection: page 1 is full (250 products) so a second
// page is fetched. status, if set, answers every request instead.
func fakeShopify(t *testing.T, status int) (*httptest.Server, *int) {
	t.Helper()
	requests := 0
	page1 := make([]map[string]any, 0, 250)
	page1 = append(page1,
		shopifyJSON("Magic the Gathering: Test Set Draft Booster Pack", "test-draft", "3.50", true),    // 22% under $4.50: a deal
		shopifyJSON("Magic the Gathering: Test Set Collector Booster", "test-collector", "9.00", true), // above market
		shopifyJSON("Magic the Gathering: Another Set Booster Pack", "another", "2.00", false),         // cheap but sold out
		shopifyJSON("Magic the Gathering: Test Set Booster Box", "test-box", "99.00", true),            // not a pack
	)
	for len(page1) < 250 {
		page1 = append(page1, shopifyJSON("Pokemon Card Sleeves", fmt.Sprintf("sleeves-%d", len(page1)), "5.00", true))
	}
	page2 := []map[string]any{{
		"title": "Magic the Gathering: Another Set Draft Booster Pack", "handle": "another-variants",
		"variants": []map[string]any{
			{"id": 11, "title": "Single Pack", "price": "2.40", "available": true},
			{"id": 12, "title": "Booster Box (36)", "price": "80.00", "available": true},
		},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		if r.URL.Path != "/collections/mtg/products.json" {
			http.NotFound(w, r)
			return
		}
		products := page1
		if r.URL.Query().Get("page") == "2" {
			products = page2
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"products": products})
	}))
	t.Cleanup(srv.Close)
	origStores, origBase, origDelay, origRetry := shopifyStores, storeBaseURL, storePageDelay, storeRetryDelay
	shopifyStores = []shopifyStore{{Name: "Fake Store", Domain: "fake.example", Collection: "mtg"}}
	storeBaseURL = func(string) string { return srv.URL }
	storePageDelay, storeRetryDelay = 0, 0
	t.Cleanup(func() {
		shopifyStores, storeBaseURL, storePageDelay, storeRetryDelay = origStores, origBase, origDelay, origRetry
	})
	return srv, &requests
}

func shopifyJSON(title, handle, price string, available bool) map[string]any {
	return map[string]any{"title": title, "handle": handle,
		"variants": []map[string]any{{"id": 1, "title": "Default Title", "price": price, "available": available}}}
}

func TestRefreshStoreOffers(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	_, requests := fakeShopify(t, 0)
	withMockClient(200, fakeSetsJSON, func() { RefreshStoreOffers(ctx, pool) })
	if *requests != 2 {
		t.Errorf("want 2 page requests, got %d", *requests)
	}
	offers, _ := db.ListStoreOffers(ctx, pool)
	got := map[string]db.Offer{}
	for _, o := range offers {
		got[o.Title] = o
	}
	if len(offers) != 4 {
		t.Fatalf("want 4 pack listings (box and sleeves skipped), got %+v", offers)
	}
	single := got["Magic the Gathering: Another Set Draft Booster Pack - Single Pack"]
	if single.MTGStocksID != 4 || single.Price != 2.40 || !strings.HasSuffix(single.URL, "/products/another-variants?variant=11") {
		t.Errorf("variant listing: %+v", single)
	}
	if d := got["Magic the Gathering: Test Set Draft Booster Pack"]; d.MTGStocksID != 1 || !d.Available || d.Store != "Fake Store" {
		t.Errorf("draft listing: %+v", d)
	}

	// The API: discounts against market, deals flagged, best first.
	h := newTestTrendsHandler(t)
	rr := httptest.NewRecorder()
	withMockClient(200, fakeSetsJSON, func() { h.StoreOffers(rr, httptest.NewRequest(http.MethodGet, "/api/store-offers", nil)) })
	var resp struct {
		Offers []Deal `json:"offers"`
		Stores []string
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Offers) != 4 || resp.Offers[0].SetName != "Another Set" || resp.Offers[0].IsDeal {
		t.Fatalf("offers: %+v", resp.Offers) // sold-out $2 pack: biggest discount, not a deal
	}
	var deals []string
	for _, o := range resp.Offers {
		if o.IsDeal {
			deals = append(deals, fmt.Sprintf("%s %.0f%%", o.Title, o.Discount*100))
		}
	}
	if len(deals) != 2 { // $3.50 vs $4.50 (22%) and $2.40 vs $3.00 (20%)
		t.Errorf("deals: %v", deals)
	}
}

func TestRefreshStoreOffers_FailureKeepsLastListing(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	_ = db.ReplaceStoreOffers(ctx, pool, "Fake Store", []db.Offer{{URL: "https://x/1", MTGStocksID: 1, Title: "kept", Price: 4, Available: true}})
	_, requests := fakeShopify(t, http.StatusTooManyRequests)
	withMockClient(200, fakeSetsJSON, func() { RefreshStoreOffers(ctx, pool) })
	if *requests != 2 {
		t.Errorf("a 429 should be retried once, then stop the store's run: got %d requests", *requests)
	}
	if offers, _ := db.ListStoreOffers(ctx, pool); len(offers) != 1 || offers[0].Title != "kept" {
		t.Errorf("last listing should be kept: %+v", offers)
	}
}

func TestNotifyAlerts_Deal(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 4}) // Another Set Draft Booster, market $3.00
	_ = db.ReplaceStoreOffers(ctx, pool, "Fake Store", []db.Offer{
		{URL: "https://fake.example/products/a", MTGStocksID: 4, Title: "Another Set Booster Pack", Price: 2.40, Available: true},
	})
	withMockClient(200, fakeSetsJSON, func() {
		if n, err := NotifyAlerts(ctx, pool); n != 1 || err != nil {
			t.Fatalf("%d %v", n, err)
		}
	})
	if ntfy.got[0].Get("Title") != "Deal: Another Set Draft Booster" ||
		ntfy.bodies[0] != "$2.40 at Fake Store, 20% under TCG market ($3.00). https://fake.example/products/a" {
		t.Errorf("deal message: %q / %q", ntfy.got[0].Get("Title"), ntfy.bodies[0])
	}
	line := alertLine(watchedAlert{product: SearchResult{SetName: "S", ProductType: "Booster Pack"}, trend: &Trend{},
		deal: &Deal{Offer: db.Offer{Store: "X", Price: 2}, Discount: 0.2}}, AlertDeal)
	if line != "$ S Booster Pack: $2.00 at X, 20% under market" {
		t.Errorf("deal line: %q", line)
	}
	// Plenty of stock mutes it.
	if got := withDealAlert(nil, &db.Watch{}, &Deal{}, muteAtStock); len(got) != 0 {
		t.Errorf("muted: %v", got)
	}
}

const ckGolden = "testdata/cardkingdom_matches.golden.txt"

// Card Kingdom's real sealed list (pack-like items, 2026-11-19) through the
// matcher: every match is recorded, so matcher changes show up as a diff.
// Regenerate with: go test ./handlers -run TestCardKingdom_RealList -update
func TestCardKingdom_RealList(t *testing.T) {
	raw, err := os.ReadFile("testdata/cardkingdom_sealed.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := gzip.NewReader(bytes.NewReader(raw))
	body, _ := io.ReadAll(zr)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	orig := cardKingdomURL
	cardKingdomURL = srv.URL
	t.Cleanup(func() { cardKingdomURL = orig })

	c, _ := loadRealCatalog(t)
	offers, n, err := fetchCardKingdom(context.Background(), newPackIndex(c.products))
	if err != nil || n < 400 {
		t.Fatalf("%d items, %v", n, err)
	}
	byID := map[int]SearchResult{}
	for _, p := range c.products {
		byID[p.MTGStocksID] = p
	}
	lines := make([]string, 0, len(offers))
	for _, o := range offers {
		p := byID[o.MTGStocksID]
		lines = append(lines, fmt.Sprintf("%s -> %s | %s", o.Title, p.SetName, p.ProductType))
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	if *updateGolden {
		if err := os.WriteFile(ckGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(ckGolden)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if got != string(want) {
		t.Errorf("Card Kingdom matches changed; review, then run with -update")
	}
	// Spot checks the user confirmed on the site.
	for _, o := range offers {
		if o.Title == "Dragon's Maze Booster Pack" && (o.Price != 3.99 || !o.Available || o.URL != "https://www.cardkingdom.com/mtg-sealed/dragons-maze-booster-pack") {
			t.Errorf("Dragon's Maze: %+v", o)
		}
	}
	for _, l := range lines {
		for _, bad := range []string{"Premium Foil", "VIP", "Special Edition"} {
			if strings.Contains(l, bad) {
				t.Errorf("not a regular pack: %s", l)
			}
		}
	}
}

func TestCardKingdom_Failures(t *testing.T) {
	idx := newPackIndex(nil)
	for _, tc := range []struct {
		status int
		body   string
	}{{http.StatusTooManyRequests, ""}, {200, "not json"}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		orig := cardKingdomURL
		cardKingdomURL = srv.URL
		if _, _, err := fetchCardKingdom(context.Background(), idx); err == nil {
			t.Errorf("status %d body %q: want an error", tc.status, tc.body)
		}
		cardKingdomURL = orig
		srv.Close()
	}
	cardKingdomURL = "http://127.0.0.1:1" // nothing listening
	defer func() { cardKingdomURL = "" }()
	if _, _, err := fetchCardKingdom(context.Background(), idx); err == nil {
		t.Error("unreachable: want an error")
	}
	cardKingdomURL = "::bad"
	if _, _, err := fetchCardKingdom(context.Background(), idx); err == nil {
		t.Error("bad URL: want an error")
	}
}

func TestRefreshStoreOffers_CardKingdom(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"base_url":"https://ck.example/"},"data":[
			{"url":"mtg-sealed/another","name":"Another Set Booster Pack","price_retail":"2.49","qty_retail":12},
			{"url":"mtg-sealed/test-box","name":"Test Set Booster Box","price_retail":"99.99","qty_retail":1},
			{"url":"mtg-sealed/test-free","name":"Test Set Draft Booster Pack","price_retail":"0","qty_retail":1}]}`))
	}))
	t.Cleanup(srv.Close)
	orig := cardKingdomURL
	cardKingdomURL = srv.URL
	t.Cleanup(func() { cardKingdomURL = orig })

	withMockClient(200, fakeSetsJSON, func() { RefreshStoreOffers(ctx, pool) })
	offers, _ := db.ListStoreOffers(ctx, pool)
	if len(offers) != 1 || offers[0].Store != "Card Kingdom" || offers[0].URL != "https://ck.example/mtg-sealed/another" || !offers[0].Available {
		t.Fatalf("offers: %+v", offers) // the box and the $0 listing are skipped
	}
	rr := httptest.NewRecorder()
	withMockClient(200, fakeSetsJSON, func() {
		newTestTrendsHandler(t).StoreOffers(rr, httptest.NewRequest(http.MethodGet, "/api/store-offers", nil))
	})
	if !strings.Contains(rr.Body.String(), `"stores":["Card Kingdom"]`) {
		t.Errorf("stores: %s", rr.Body.String())
	}
}

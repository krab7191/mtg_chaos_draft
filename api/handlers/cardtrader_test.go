package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

// fakeCardTrader serves expansions, blueprints and listings shaped like the
// real API (from real responses, 2026-11-19). refuseFirst answers the first
// request with 429.
func fakeCardTrader(t *testing.T, refuseFirst bool) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	var refused atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if refuseFirst && refused.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		switch {
		case r.URL.Path == "/expansions":
			_, _ = w.Write([]byte(`[{"id":1,"game_id":1,"code":"tst","name":"Test Set"},
				{"id":2,"game_id":1,"code":"zzz","name":"Not In Catalog"},
				{"id":3,"game_id":4,"code":"tst","name":"Test Set"}]`))
		case r.URL.Path == "/blueprints/export" && r.URL.Query().Get("expansion_id") == "1":
			_, _ = w.Write([]byte(`[{"id":10,"name":"Test Set Draft Booster","category_id":5},
				{"id":11,"name":"Test Set Six Card Booster","category_id":5},
				{"id":12,"name":"Test Set Booster Box","category_id":4},
				{"id":13,"name":"Lightning Bolt","category_id":1}]`))
		case r.URL.Path == "/marketplace/products" && r.URL.Query().Get("blueprint_id") == "10":
			_, _ = w.Write([]byte(`{"10":[
				{"price_cents":495,"price_currency":"USD","quantity":9,"on_vacation":false,"properties_hash":{"mtg_language":"jp"},"user":{"country_code":"BE"}},
				{"price_cents":450,"price_currency":"USD","quantity":2,"on_vacation":false,"properties_hash":{"mtg_language":"jp"},"user":{"country_code":"IT"}},
				{"price_cents":523,"price_currency":"USD","quantity":1,"on_vacation":false,"properties_hash":{"mtg_language":"en"},"user":{"country_code":"CA"}},
				{"price_cents":100,"price_currency":"USD","quantity":1,"on_vacation":true,"properties_hash":{"mtg_language":"en"},"user":{"country_code":"US"}},
				{"price_cents":200,"price_currency":"EUR","quantity":1,"on_vacation":false,"properties_hash":{"mtg_language":"de"},"user":{"country_code":"DE"}},
				{"price_cents":300,"price_currency":"USD","quantity":0,"on_vacation":false,"properties_hash":{"mtg_language":"kr"},"user":{"country_code":"KR"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	origBase, origToken, origDelay, origRetry := cardTraderBase, cardTraderToken, cardTraderDelay, cardTraderRetry
	cardTraderBase, cardTraderToken, cardTraderDelay, cardTraderRetry = srv.URL, "test-token", 0, 0
	t.Cleanup(func() {
		cardTraderBase, cardTraderToken, cardTraderDelay, cardTraderRetry = origBase, origToken, origDelay, origRetry
	})
	return &n
}

func ctDB(t *testing.T) context.Context {
	t.Helper()
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "cardtrader_expansions", "cardtrader_blueprints")
	if _, err := pool.Exec(ctx, `TRUNCATE cardtrader_expansions, cardtrader_blueprints`); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestCardTrader_RefreshKeepsCheapestPerLanguage(t *testing.T) {
	ctx := ctDB(t)
	pool := testhelper.Pool(t)
	requests := fakeCardTrader(t, true) // the first request is refused, then retried
	withMockClient(200, fakeSetsJSON, func() { RefreshStoreOffers(ctx, pool) })

	offers, _ := db.ListStoreOffers(ctx, pool)
	got := map[string]db.Offer{}
	for _, o := range offers {
		got[o.Language] = o
	}
	if len(offers) != 2 {
		t.Fatalf("want an English and a Japanese listing, got %+v", offers)
	}
	ja := got["ja"]
	if ja.Price != 4.50 || ja.MTGStocksID != 1 || ja.Store != "CardTrader" || !strings.Contains(ja.Title, "Japanese · ships from IT") ||
		ja.URL != "https://www.cardtrader.com/en/cards/10#ja" {
		t.Errorf("Japanese: %+v", ja)
	}
	if en := got["en"]; en.Price != 5.23 || !strings.Contains(en.Title, "English · ships from CA") {
		t.Errorf("English (the $1 one is on vacation): %+v", en)
	}
	// 429 + expansions + one set's blueprints + one booster's listings.
	if n := requests.Load(); n != 4 {
		t.Errorf("requests: %d", n)
	}
	// The next day: sets aren't scanned again, only listings are read.
	requests.Store(0)
	withMockClient(200, fakeSetsJSON, func() { RefreshStoreOffers(ctx, pool) })
	if n := requests.Load(); n != 2 { // expansions + listings
		t.Errorf("second day requests: %d", n)
	}
}

func TestCardTrader_Failures(t *testing.T) {
	ctx := ctDB(t)
	pool := testhelper.Pool(t)
	c, _ := parseCatalog([]byte(fakeSetsJSON))
	idx := newPackIndex(c.products)
	fakeCardTrader(t, false)

	cardTraderToken = "wrong"
	if _, _, err := fetchCardTrader(ctx, pool, c, idx); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("bad token: %v", err)
	}
	cardTraderToken = "test-token"
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := fetchCardTrader(cancelled, pool, c, idx); err == nil {
		t.Error("cancelled: want an error")
	}
	if _, _, err := fetchCardTrader(ctx, closedPool(t), c, idx); err == nil {
		t.Error("db down: want an error")
	}
	// Garbage JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("nope")) }))
	defer srv.Close()
	cardTraderBase = srv.URL
	if _, _, err := fetchCardTrader(ctx, pool, c, idx); err == nil {
		t.Error("bad JSON: want an error")
	}
	cardTraderBase = "::bad"
	if _, _, err := fetchCardTrader(ctx, pool, c, idx); err == nil {
		t.Error("bad URL: want an error")
	}
	cardTraderBase = "http://127.0.0.1:1"
	if _, _, err := fetchCardTrader(ctx, pool, c, idx); err == nil {
		t.Error("unreachable: want an error")
	}
}

func TestLanguages(t *testing.T) {
	cases := map[string]string{
		"Magic the Gathering: Prophecy Booster Pack":            "en",
		"Magic the Gathering: Prophecy Booster Pack (Japanese)": "ja",
		"(1Pack)《Dragon's Maze Booster Pack[JPN]》[DGM]":         "ja",
		"Mirage Booster Pack - German":                          "de",
		"Ice Age Booster Pack (Foreign)":                        "other",
	}
	for title, want := range cases {
		if got := languageOf(title); got != want {
			t.Errorf("%q: %q, want %q", title, got, want)
		}
	}
	for in, want := range map[string]string{"": "en", "jp": "ja", "kr": "ko", "zh-TW": "zh", "it": "it"} {
		if got := ctLanguage(in); got != want {
			t.Errorf("ct %q: %q, want %q", in, got, want)
		}
	}
	ConfigureCardTrader(" tok ")
	if cardTraderToken != "tok" {
		t.Errorf("token: %q", cardTraderToken)
	}
	ConfigureCardTrader("")
}

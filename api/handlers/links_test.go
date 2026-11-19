package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestDirectTCGURL(t *testing.T) {
	cases := map[string]string{
		// MTGStocks' referral link (real, from /sealed/849).
		"https://partner.tcgplayer.com/jee3n6?u=https%3A%2F%2Fwww.tcgplayer.com%2Fproduct%2F67752%2Fmagic-dragons-maze-dragons-maze-booster-pack&subId2=sealed": "https://www.tcgplayer.com/product/67752/magic-dragons-maze-dragons-maze-booster-pack",
		"https://www.tcgplayer.com/product/1/x?foo=bar":               "https://www.tcgplayer.com/product/1/x",
		"https://partner.tcgplayer.com/x?u=https%3A%2F%2Fevil.com%2F": "", // only TCGplayer pages
		"https://example.com/product/1":                               "",
		"":                                                            "",
		"::not a url":                                                 "",
	}
	for in, want := range cases {
		if got := directTCGURL(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func resetTCGLinks(t *testing.T) {
	t.Helper()
	tcgLinksMu.Lock()
	tcgLinks = map[int]string{}
	tcgLinksMu.Unlock()
	t.Cleanup(func() { tcgLinksMu.Lock(); tcgLinks = map[int]string{}; tcgLinksMu.Unlock() })
}

func TestBackfillTCGLinks(t *testing.T) {
	ctx := context.Background()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "product_links")
	if _, err := pool.Exec(ctx, `TRUNCATE product_links`); err != nil {
		t.Fatal(err)
	}
	resetTCGLinks(t)
	orig := backfillDelay
	backfillDelay = 0
	t.Cleanup(func() { backfillDelay = orig })
	c, _ := parseCatalog([]byte(fakeSetsJSON)) // packs 1, 2, 4

	body := `{"id":1,"tcgUrl":"https://partner.tcgplayer.com/a?u=https%3A%2F%2Fwww.tcgplayer.com%2Fproduct%2F9%2Fpack&subId2=sealed"}`
	withMockClient(200, body, func() {
		if n, err := backfillTCGLinks(ctx, pool, c); n != 3 || err != nil {
			t.Fatalf("first run: %d %v", n, err)
		}
		if n, _ := backfillTCGLinks(ctx, pool, c); n != 0 {
			t.Errorf("second run should fetch nothing, got %d", n)
		}
	})
	if tcgLink(1) != "https://www.tcgplayer.com/product/9/pack" {
		t.Errorf("link: %q", tcgLink(1))
	}
	// Stored, so a restart loads them without asking again.
	tcgLinksMu.Lock()
	tcgLinks = map[int]string{}
	tcgLinksMu.Unlock()
	if err := LoadProductLinks(ctx, pool); err != nil || tcgLink(4) == "" {
		t.Errorf("loaded: %q %v", tcgLink(4), err)
	}
	if err := LoadProductLinks(ctx, closedPool(t)); err == nil {
		t.Error("db down should fail")
	}

	// Cheapest packs carry the link.
	rr := httptest.NewRecorder()
	t.Cleanup(resetProductsCache)
	withMockClient(200, fakeSetsJSON, func() {
		resetProductsCache()
		CheapestPacks(rr, httptest.NewRequest(http.MethodGet, "/api/cheapest", nil))
	})
	var cheapest []SearchResult
	_ = json.NewDecoder(rr.Body).Decode(&cheapest)
	if len(cheapest) == 0 || cheapest[0].TCGURL != "https://www.tcgplayer.com/product/9/pack" {
		t.Errorf("cheapest: %+v", cheapest)
	}
}

func TestBackfillTCGLinks_Failures(t *testing.T) {
	ctx := context.Background()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "product_links")
	if _, err := pool.Exec(ctx, `TRUNCATE product_links`); err != nil {
		t.Fatal(err)
	}
	resetTCGLinks(t)
	orig := backfillDelay
	backfillDelay = 0
	t.Cleanup(func() { backfillDelay = orig })
	c, _ := parseCatalog([]byte(fakeSetsJSON))

	// Rate-limited: stop at once, nothing stored.
	withMockClient(429, "", func() {
		if n, err := backfillTCGLinks(ctx, pool, c); n != 0 || err == nil {
			t.Errorf("429: %d %v", n, err)
		}
	})
	withMockClient(200, "not json", func() {
		if _, err := backfillTCGLinks(ctx, pool, c); err == nil {
			t.Error("bad JSON should fail")
		}
	})
	withErrClient(func() {
		if _, err := backfillTCGLinks(ctx, pool, c); err == nil {
			t.Error("network error should fail")
		}
	})
	// Unknown product: stored as "none" and not asked again.
	withMockClient(404, "", func() {
		if n, err := backfillTCGLinks(ctx, pool, c); n != 3 || err != nil {
			t.Errorf("404: %d %v", n, err)
		}
	})
	if links, _ := db.ProductLinks(ctx, pool); len(links) != 3 || links[1] != "" {
		t.Errorf("stored: %v", links)
	}
	// A cancelled run stops before asking.
	resetTCGLinks(t)
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	backfillDelay = 1e9
	if _, err := backfillTCGLinks(cctx, pool, c); err == nil {
		t.Error("cancelled: want an error")
	}
	if err := db.SetProductLink(ctx, closedPool(t), 1, ""); err == nil {
		t.Error("db down should fail")
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

// blockedCounter answers every request the same way and counts them.
type blockedCounter struct {
	status int
	body   string
	calls  atomic.Int32
}

func (c *blockedCounter) RoundTrip(r *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return (&stubTransport{statusCode: c.status, body: c.body}).RoundTrip(r)
}

func withTransport(rt http.RoundTripper, f func()) {
	orig := mtgstocksClient
	mtgstocksClient = &http.Client{Transport: rt}
	resetMTGStocksBackoff()
	defer func() { mtgstocksClient = orig; resetMTGStocksBackoff() }()
	f()
}

// withCatalogStore points the catalog at the test DB, emptied, for one test.
func withCatalogStore(t *testing.T) {
	t.Helper()
	pool := testhelper.Pool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE mtgstocks_catalog`); err != nil {
		t.Fatal(err)
	}
	resetProductsCache()
	UseCatalogStore(pool)
	t.Cleanup(func() {
		UseCatalogStore(nil)
		resetProductsCache()
		_, _ = pool.Exec(context.Background(), `TRUNCATE mtgstocks_catalog`)
	})
}

func TestParseCatalog_PricesEveryProduct(t *testing.T) {
	c, err := parseCatalog([]byte(fakeSetsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.products) != 3 {
		t.Errorf("search products: want 3, got %d", len(c.products))
	}
	// Product 1 prefers market over average.
	if c.prices[1] != 4.50 || c.prices[2] != 8.00 || c.prices[4] != 3.00 {
		t.Errorf("prices: %v", c.prices)
	}
	if _, ok := c.prices[3]; ok {
		t.Error("unpriced bundle should have no price")
	}
}

func TestGetCatalog_SavesLiveAndFallsBackToSaved(t *testing.T) {
	withCatalogStore(t)

	// A good live fetch is persisted.
	withMockClient(200, fakeSetsJSON, func() {
		if _, err := getCatalog(); err != nil {
			t.Fatalf("live: %v", err)
		}
	})
	saved, err := db.LoadCatalog(context.Background(), catalogPool)
	if err != nil || saved.Source != "live" {
		t.Fatalf("want saved live catalog, got %v %v", saved, err)
	}

	// After a restart (empty memory), a blocked fetch falls back to it.
	resetProductsCache()
	withMockClient(403, "", func() {
		products, err := getMTGStocksProducts()
		if err != nil {
			t.Fatalf("fallback: %v", err)
		}
		if len(products) != 3 {
			t.Errorf("want 3 products from saved catalog, got %d", len(products))
		}
	})
}

func TestGetCatalog_ErrorsWithNothingSaved(t *testing.T) {
	withCatalogStore(t)
	withMockClient(403, "", func() {
		if _, err := getCatalog(); err == nil || !strings.Contains(err.Error(), "403") {
			t.Errorf("want 403 error, got %v", err)
		}
	})
}

func TestGetCatalog_DoesNotHammerWhenBlocked(t *testing.T) {
	resetProductsCache()
	t.Cleanup(resetProductsCache)
	withMockClient(200, fakeSetsJSON, func() {
		if _, err := getCatalog(); err != nil {
			t.Fatal(err)
		}
	})
	// Force the next call to try live again, which is blocked.
	catalogMu.Lock()
	lastLiveAttempt = time.Now().Add(-2 * catalogLiveInterval)
	catalogMu.Unlock()

	blocked := &blockedCounter{status: 403}
	withTransport(blocked, func() {
		for i := 0; i < 5; i++ {
			if _, err := getCatalog(); err != nil {
				t.Fatalf("stale catalog should be served: %v", err)
			}
		}
	})
	if n := blocked.calls.Load(); n != 1 {
		t.Errorf("want 1 live attempt per interval, got %d", n)
	}
}

func TestFetchSealedPrice_FallsBackToCatalog(t *testing.T) {
	resetProductsCache()
	t.Cleanup(resetProductsCache)
	withMockClient(200, fakeSetsJSON, func() {
		if _, err := getCatalog(); err != nil {
			t.Fatal(err)
		}
	})
	withMockClient(403, "", func() {
		if price, ok := fetchSealedPrice(2); !ok || price != 8.00 {
			t.Errorf("want cached 8.00, got %v %v", price, ok)
		}
		if _, ok := fetchSealedPrice(999); ok {
			t.Error("unknown product should have no price")
		}
	})
}

func TestCatalogUpload(t *testing.T) {
	withCatalogStore(t)
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	old := 1.00
	pack, err := db.AddPack(context.Background(), pool, 2, "Test Collector Booster", "Test Set", "TST", "collector_booster", 1, 1.0, &old, 15)
	if err != nil {
		t.Fatal(err)
	}
	h := NewCatalogHandler(pool)

	put := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.Upload(rr, httptest.NewRequest(http.MethodPut, "/api/catalog", strings.NewReader(body)))
		return rr
	}

	for _, bad := range []string{"not json", "[]", `{"a":1}`} {
		if rr := put(bad); rr.Code != http.StatusBadRequest {
			t.Errorf("%q: want 400, got %d", bad, rr.Code)
		}
	}

	rr := put(fakeSetsJSON)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload: want 200, got %d %s", rr.Code, rr.Body)
	}
	var st catalogStatus
	if err := json.NewDecoder(rr.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if !st.Available || st.Source != "upload" || st.Products != 3 || st.FetchedAt == nil {
		t.Errorf("status: %+v", st)
	}

	// The collection picked up the snapshot's price.
	packs, err := db.GetPacksByIDs(context.Background(), pool, []int{pack.ID})
	if err != nil || len(packs) != 1 || packs[0].MarketPrice == nil || *packs[0].MarketPrice != 8.00 {
		t.Errorf("pack price not updated: %+v %v", packs, err)
	}

	// Search now works without MTGStocks.
	withMockClient(403, "", func() {
		rr := httptest.NewRecorder()
		h.Status(rr, httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
		if !strings.Contains(rr.Body.String(), `"source":"upload"`) {
			t.Errorf("status after upload: %s", rr.Body)
		}
	})
}

func TestCatalogStatus_NoneAvailable(t *testing.T) {
	withCatalogStore(t)
	h := NewCatalogHandler(testhelper.Pool(t))
	withMockClient(403, "", func() {
		rr := httptest.NewRecorder()
		h.Status(rr, httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
		if !strings.Contains(rr.Body.String(), `"available":false`) {
			t.Errorf("want unavailable, got %s", rr.Body)
		}
	})
}

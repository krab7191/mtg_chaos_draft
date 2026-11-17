package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/testhelper"
)

// ── CheapestPacks ─────────────────────────────────────────────────────────────

func TestCheapestPacks_Success(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/cheapest-packs", nil)
		rr := httptest.NewRecorder()
		CheapestPacks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(results) == 0 {
			t.Error("want non-empty results, got none")
		}
		// verify sorted ascending by price
		for i := 1; i < len(results); i++ {
			if *results[i].MarketPrice < *results[i-1].MarketPrice {
				t.Errorf("results not sorted: index %d (%v) < index %d (%v)",
					i, *results[i].MarketPrice, i-1, *results[i-1].MarketPrice)
			}
		}
	})
}

func TestCheapestPacks_BeforeFilter(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		// TST date is 2023-01-01, ANS date is 2022-06-01.
		// ?before=2022-12-31 should only include ANS products.
		req := httptest.NewRequest(http.MethodGet, "/api/cheapest-packs?before=2022-12-31", nil)
		rr := httptest.NewRecorder()
		CheapestPacks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, r := range results {
			if r.SetCode != "ANS" {
				t.Errorf("expected only ANS products, got set code %q", r.SetCode)
			}
		}
		if len(results) == 0 {
			t.Error("want ANS results, got none")
		}
	})
}

func TestCheapestPacks_InvalidBefore(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/cheapest-packs?before=notadate", nil)
		rr := httptest.NewRecorder()
		CheapestPacks(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", rr.Code)
		}
	})
}

func TestCheapestPacks_NetworkError(t *testing.T) {
	resetProductsCache()
	withErrClient(func() {
		req := httptest.NewRequest(http.MethodGet, "/api/cheapest-packs", nil)
		rr := httptest.NewRecorder()
		CheapestPacks(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Errorf("want 502, got %d", rr.Code)
		}
	})
}

func TestCheapestPacks_EmptyWhenNoPrices(t *testing.T) {
	resetProductsCache()
	const noPriceJSON = `[{"name":"No Price Set","abbreviation":"NPS","date":"2023-01-01","products":[{"id":99,"name":"No Price Pack","type":"draft_booster","latestPrice":null}]}]`
	withMockClient(200, noPriceJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/cheapest-packs", nil)
		rr := httptest.NewRecorder()
		CheapestPacks(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("want empty array, got %d results", len(results))
		}
	})
}

// ── RefreshPrices ─────────────────────────────────────────────────────────────

func TestRefreshPrices_Success(t *testing.T) {
	resetProductsCache()
	pool := testhelper.Pool(t)
	withMockClient(200, fakeSetsJSON, func() {
		// should not panic
		RefreshPrices(context.Background(), pool)
	})
}

func TestRefreshPrices_FetchError(t *testing.T) {
	resetProductsCache()
	pool := testhelper.Pool(t)
	withErrClient(func() {
		// should not panic; function logs and returns
		RefreshPrices(context.Background(), pool)
	})
}

func TestCheapestPacks_ExcludesSleeved(t *testing.T) {
	resetProductsCache()
	t.Cleanup(resetProductsCache)
	const sets = `[{"name":"Eldritch Moon","abbreviation":"EMN","date":"2016-07-22","products":[
	  {"id":1,"name":"Sleeved Booster Pack","type":null,"latestPrice":{"market":5.00}},
	  {"id":2,"name":"Booster Pack","type":null,"latestPrice":{"market":14.84}}]}]`
	withMockClient(200, sets, func() {
		rr := httptest.NewRecorder()
		CheapestPacks(rr, httptest.NewRequest(http.MethodGet, "/api/cheapest", nil))
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].MTGStocksID != 2 {
			t.Errorf("want only the regular booster, got %+v", results)
		}
	})
}

// TestCheapestPacks_RealCatalogHasNoSleeved checks the rule against every
// set in the real MTGStocks snapshot.
func TestCheapestPacks_RealCatalogHasNoSleeved(t *testing.T) {
	c, _ := loadRealCatalog(t)
	catalogMu.Lock()
	currentCatalog, lastLiveAttempt = c, time.Now()
	catalogMu.Unlock()
	t.Cleanup(resetProductsCache)

	rr := httptest.NewRecorder()
	CheapestPacks(rr, httptest.NewRequest(http.MethodGet, "/api/cheapest", nil))
	var results []SearchResult
	if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 {
		t.Fatalf("want a full top 10, got %d", len(results))
	}
	for _, r := range results {
		if strings.HasPrefix(r.ProductType, "Sleeved") {
			t.Errorf("sleeved pack listed: %s %s", r.SetName, r.ProductType)
		}
	}
}

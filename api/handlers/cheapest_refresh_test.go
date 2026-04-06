package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

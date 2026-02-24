package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeSetsJSON is a minimal MTGStocks /sealed response used across tests.
const fakeSetsJSON = `[
  {
    "name": "Test Set",
    "abbreviation": "TST",
    "date": "2023-01-01",
    "products": [
      {"id": 1, "name": "Test Draft Booster Pack", "type": "draft_booster",
       "latestPrice": {"average": 5.00, "market": 4.50}},
      {"id": 2, "name": "Test Collector Booster", "type": "collector_booster",
       "latestPrice": {"average": null, "market": 8.00}},
      {"id": 3, "name": "Test Bundle", "type": "bundle", "latestPrice": null}
    ]
  },
  {
    "name": "Another Set",
    "abbreviation": "ANS",
    "date": "2022-06-01",
    "products": [
      {"id": 4, "name": "Another Draft Booster Pack", "type": "draft_booster",
       "latestPrice": {"average": 3.50, "market": 3.00}}
    ]
  }
]`

func resetProductsCache() {
	productsCacheMu.Lock()
	productsCache = nil
	productsCacheTime = time.Time{}
	productsCacheMu.Unlock()
}

// ── getMTGStocksProducts ───────────────────────────────────────────────────────

func TestGetMTGStocksProducts_Success(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		products, err := getMTGStocksProducts()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// bundle is excluded; draft + collector from TST, draft from ANS = 3
		if len(products) != 3 {
			t.Errorf("want 3 products, got %d", len(products))
		}
	})
}

func TestGetMTGStocksProducts_CacheHit(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		p1, err := getMTGStocksProducts()
		if err != nil {
			t.Fatalf("first call: %v", err)
		}
		p2, err := getMTGStocksProducts()
		if err != nil {
			t.Fatalf("second call: %v", err)
		}
		if len(p1) != len(p2) {
			t.Errorf("cache returned different results: %d vs %d", len(p1), len(p2))
		}
	})
}

func TestGetMTGStocksProducts_NetworkError(t *testing.T) {
	resetProductsCache()
	withErrClient(func() {
		_, err := getMTGStocksProducts()
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

func TestGetMTGStocksProducts_Non200(t *testing.T) {
	resetProductsCache()
	withMockClient(503, `{}`, func() {
		_, err := getMTGStocksProducts()
		if err == nil {
			t.Fatal("want error for non-200 status")
		}
	})
}

func TestGetMTGStocksProducts_BadJSON(t *testing.T) {
	resetProductsCache()
	withMockClient(200, `not json`, func() {
		_, err := getMTGStocksProducts()
		if err == nil {
			t.Fatal("want error for bad JSON")
		}
	})
}

func TestGetMTGStocksProducts_PriceFields(t *testing.T) {
	resetProductsCache()
	// market price used when average is null
	withMockClient(200, fakeSetsJSON, func() {
		products, err := getMTGStocksProducts()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// find TST collector booster (id=2): average null, market=8.00
		for _, p := range products {
			if p.MTGStocksID == 2 {
				if p.MarketPrice == nil || *p.MarketPrice != 8.00 {
					t.Errorf("want market price 8.00, got %v", p.MarketPrice)
				}
				return
			}
		}
		t.Error("product id=2 not found")
	})
}

// ── Search handler ────────────────────────────────────────────────────────────

func TestSearch_ShortQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=a", nil)
	rr := httptest.NewRecorder()
	Search(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if rr.Body.String() != "[]" {
		t.Errorf("want '[]', got %q", rr.Body.String())
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	rr := httptest.NewRecorder()
	Search(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if rr.Body.String() != "[]" {
		t.Errorf("want '[]', got %q", rr.Body.String())
	}
}

func TestSearch_Found_ShortQuery(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=Test", nil)
		rr := httptest.NewRecorder()
		Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(results) == 0 {
			t.Error("want results, got none")
		}
	})
}

func TestSearch_Found_LongQueryFuzzy(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=Test+Set+Draft", nil)
		rr := httptest.NewRecorder()
		Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(results) == 0 {
			t.Error("want fuzzy results, got none")
		}
	})
}

func TestSearch_NoMatches(t *testing.T) {
	resetProductsCache()
	withMockClient(200, fakeSetsJSON, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=zzz", nil)
		rr := httptest.NewRecorder()
		Search(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rr.Code)
		}
		var results []SearchResult
		_ = json.NewDecoder(rr.Body).Decode(&results)
		if len(results) != 0 {
			t.Errorf("want 0 results, got %d", len(results))
		}
	})
}

func TestSearch_ProxyError(t *testing.T) {
	resetProductsCache()
	withErrClient(func() {
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", nil)
		rr := httptest.NewRecorder()
		Search(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Errorf("want 502, got %d", rr.Code)
		}
	})
}

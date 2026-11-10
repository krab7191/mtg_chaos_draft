package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func seedPriceTestPack(t *testing.T, price float64) *db.CollectionPack {
	t.Helper()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	pack, err := db.AddPack(context.Background(), pool, 40401, "Pack", "Set", "", "draft_booster", 3, 1.0, &price, 15)
	if err != nil {
		t.Fatalf("seed pack: %v", err)
	}
	return pack
}

func callRefreshPrice(t *testing.T, id string) (*httptest.ResponseRecorder, db.CollectionPack) {
	t.Helper()
	h := NewCollectionHandler(testhelper.Pool(t))
	req := httptest.NewRequest(http.MethodPost, "/api/collection/x/refresh-price", nil)
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h.RefreshPrice(rr, req)
	var pack db.CollectionPack
	if rr.Code == http.StatusOK {
		if err := json.NewDecoder(rr.Body).Decode(&pack); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rr, pack
}

func TestRefreshPrice_UpdatesWithLivePrice(t *testing.T) {
	pack := seedPriceTestPack(t, 4.00)
	withMockClient(200, `{"latestPrice":{"average":6.25}}`, func() {
		rr, got := callRefreshPrice(t, strconv.Itoa(pack.ID))
		if rr.Code != http.StatusOK {
			t.Fatalf("status: want 200, got %d — %s", rr.Code, rr.Body)
		}
		if got.MarketPrice == nil || *got.MarketPrice != 6.25 {
			t.Errorf("marketPrice: want 6.25, got %v", got.MarketPrice)
		}
	})
}

func TestRefreshPrice_FallsBackOnFetchError(t *testing.T) {
	pack := seedPriceTestPack(t, 4.00)
	withErrClient(func() {
		rr, got := callRefreshPrice(t, strconv.Itoa(pack.ID))
		if rr.Code != http.StatusOK {
			t.Fatalf("status: want 200, got %d", rr.Code)
		}
		if got.MarketPrice == nil || *got.MarketPrice != 4.00 {
			t.Errorf("marketPrice: want stored 4.00, got %v", got.MarketPrice)
		}
	})
}

func TestRefreshPrice_NotFound(t *testing.T) {
	seedPriceTestPack(t, 4.00)
	rr, _ := callRefreshPrice(t, "999999")
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestRefreshPrice_BadID(t *testing.T) {
	rr, _ := callRefreshPrice(t, "abc")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// ── LinkPrice ────────────────────────────────────────────────────────────────

func callLinkPrice(t *testing.T, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewCollectionHandler(testhelper.Pool(t))
	req := httptest.NewRequest(http.MethodPost, "/api/collection/x/link-price", strings.NewReader(body))
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h.LinkPrice(rr, req)
	return rr
}

func TestLinkPrice_StoresIDAndLivePrice(t *testing.T) {
	pack := seedPriceTestPack(t, 4.00)
	withMockClient(200, `{"latestPrice":{"low":3.10,"market":6.10,"average":6.25}}`, func() {
		rr := callLinkPrice(t, strconv.Itoa(pack.ID), `{"mtgstocksId":50505}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: want 200, got %d — %s", rr.Code, rr.Body)
		}
		var got db.CollectionPack
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.MTGStocksID == nil || *got.MTGStocksID != 50505 {
			t.Errorf("mtgstocksId: want 50505, got %v", got.MTGStocksID)
		}
		if got.MarketPrice == nil || *got.MarketPrice != 6.10 {
			t.Errorf("price: want market 6.10, got %v", got.MarketPrice)
		}
	})
}

func TestLinkPrice_KeepsOldPriceWhenFetchFails(t *testing.T) {
	pack := seedPriceTestPack(t, 4.00)
	withErrClient(func() {
		rr := callLinkPrice(t, strconv.Itoa(pack.ID), `{"mtgstocksId":50506}`)
		var got db.CollectionPack
		_ = json.NewDecoder(rr.Body).Decode(&got)
		if rr.Code != http.StatusOK || got.MarketPrice == nil || *got.MarketPrice != 4.00 {
			t.Errorf("want 200 with stored price 4.00, got %d %v", rr.Code, got.MarketPrice)
		}
	})
}

func TestLinkPrice_BadRequests(t *testing.T) {
	cases := map[string]struct{ id, body string }{
		"bad id":       {"abc", `{"mtgstocksId":1}`},
		"bad json":     {"1", `nope`},
		"missing id":   {"1", `{}`},
		"zero mtgs id": {"1", `{"mtgstocksId":0}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rr := callLinkPrice(t, tc.id, tc.body); rr.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rr.Code)
			}
		})
	}
}

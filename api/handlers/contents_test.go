package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func resetContents(t *testing.T) {
	t.Helper()
	clear := func() { setContents(nil, map[string]cardPrice{}, map[string]string{}) }
	clear()
	t.Cleanup(clear)
}

func getContents(t *testing.T, h *TrendsHandler, id string) (int, Breakdown, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/packs/"+id+"/contents", nil)
	req.SetPathValue("mtgstocksId", id)
	withMockClient(200, fakeSetsJSON, func() { h.Contents(rr, req) })
	var b Breakdown
	_ = json.Unmarshal(rr.Body.Bytes(), &b)
	return rr.Code, b, rr.Body.String()
}

func TestContents(t *testing.T) {
	ctx := evDB(t)
	pool := testhelper.Pool(t)
	resetContents(t)
	h := newTestTrendsHandler(t)

	// Before the first EV run after a restart.
	if code, _, body := getContents(t, h, "1"); code != 404 || !strings.Contains(body, "still loading") {
		t.Errorf("before loading: %d %s", code, body)
	}

	fakeMTGJSON(t)
	withMockClient(200, fakeSetsJSON, func() {
		if err := RefreshEV(ctx, pool); err != nil {
			t.Fatal(err)
		}
	})
	// TST draft (id 1, market $4.50) at the latest prices: main slot 3/4 x $2
	// + 1/4 x $4 = $2.50, foil slot $12 (etched) = $14.50.
	code, b, body := getContents(t, h, "1")
	if code != 200 || math.Abs(b.Value-14.5) > 1e-9 || b.Price != 4.5 || b.Premium || b.Cards != 2 {
		t.Fatalf("draft: %d %s", code, body)
	}
	if len(b.Slots) != 2 || b.Slots[0].Name != "shiny" || math.Abs(b.Slots[0].Value-12) > 1e-9 {
		t.Errorf("slots: %+v", b.Slots)
	}
	if len(b.Top) != 3 || b.Top[0].Name != "Shiny Card" || !b.Top[0].Foil || b.Top[0].Chance != 1 ||
		b.Top[1].Name != "Alpha Card" || math.Abs(b.Top[1].Chance-0.75) > 1e-9 {
		t.Errorf("top: %+v", b.Top)
	}
	// The collector pack ($8.00) holds two $12 foils: not a premium.
	if _, c, _ := getContents(t, h, "2"); c.Value != 24 || c.Premium || math.Abs(c.Ratio-8.0/24) > 1e-9 {
		t.Errorf("collector: %+v", c)
	}
	// A set without pull rates, a bad id, a product that isn't a pack.
	if code, _, body := getContents(t, h, "4"); code != 404 || !strings.Contains(body, "no pull-rate data") {
		t.Errorf("no data: %d %s", code, body)
	}
	if code, _, _ := getContents(t, h, "x"); code != 400 {
		t.Errorf("bad id: %d", code)
	}
	if code, _, _ := getContents(t, h, "999"); code != 404 {
		t.Errorf("unknown: %d", code)
	}

	// Listings carry the contents value; one at 3x+ the contents is a premium.
	listing := []db.Offer{{Store: "S", URL: "u", MTGStocksID: 1, Title: "Test Set Draft Booster Pack", Price: 8, Available: true}}
	deals := offersWithMarket(listing, mustCatalog(t).products)
	if len(deals) != 1 || deals[0].Contents != 14.5 || deals[0].Premium {
		t.Errorf("listing: %+v", deals)
	}
	contentsMu.Lock()
	contentsValue[1] = 2 // pretend the cards inside crashed
	contentsMu.Unlock()
	if deals := offersWithMarket(listing, mustCatalog(t).products); len(deals) != 1 || !deals[0].Premium {
		t.Errorf("premium: %+v", deals)
	}
	if slotName("foilRareMythic") != "foil rare mythic" {
		t.Errorf("slot name: %q", slotName("foilRareMythic"))
	}
}

func mustCatalog(t *testing.T) *catalog {
	t.Helper()
	c, err := parseCatalog([]byte(fakeSetsJSON))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestContents_NoCatalog(t *testing.T) {
	trendsDB(t)
	h := newTestTrendsHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/packs/1/contents", nil)
	req.SetPathValue("mtgstocksId", "1")
	withMockClient(403, "", func() { h.Contents(rr, req) })
	if rr.Code != http.StatusBadGateway {
		t.Errorf("no catalog: %d", rr.Code)
	}
}

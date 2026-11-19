package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

type bestResp struct {
	Candidates []Candidate                   `json:"candidates"`
	Shipping   map[string]float64            `json:"shipping"`
	Timing     map[string]map[Signal]float64 `json:"timing"`
}

func getBest(t *testing.T, h *TrendsHandler) (int, bestResp) {
	t.Helper()
	rr := httptest.NewRecorder()
	withMockClient(200, fakeSetsJSON, func() { h.BestDeals(rr, httptest.NewRequest(http.MethodGet, "/api/best-deals", nil)) })
	var b bestResp
	_ = json.Unmarshal(rr.Body.Bytes(), &b)
	return rr.Code, b
}

func TestBestDeals(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	resetContents(t)
	// Contents values: pack 1 (Test Set Draft, market $4.50) $6; pack 4 (Another
	// Set Draft, $3.00) $3; pack 2 (Collector, $8) none -> left out.
	contentsMu.Lock()
	contentsValue = map[int]float64{1: 6, 4: 3}
	contentsMu.Unlock()
	_ = db.ReplaceStoreOffers(ctx, pool, "CardTrader", []db.Offer{
		{URL: "ct-1", MTGStocksID: 1, Title: "Test Set Draft Booster · Italian · ships from IT", Price: 3, Available: true, Language: "it"},
		{URL: "ct-1-gone", MTGStocksID: 1, Title: "Test Set Draft Booster · English · ships from US", Price: 1, Available: false},
	})

	code, b := getBest(t, newTestTrendsHandler(t))
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(b.Candidates) != 3 { // TCGplayer for packs 1 and 4, plus the in-stock CardTrader listing
		t.Fatalf("candidates: %+v", b.Candidates)
	}
	top := b.Candidates[0]
	if top.Store != "CardTrader" || top.Price != 3 || top.Language != "it" || top.Contents != 6 || top.Market != 4.5 {
		t.Errorf("best first: %+v", top)
	}
	for _, c := range b.Candidates {
		if c.MTGStocksID == 2 {
			t.Error("a pack without a contents value can't be scored")
		}
		if c.Store == "TCGplayer" && (c.Price != c.Market || c.Language != "en") {
			t.Errorf("TCGplayer candidate: %+v", c)
		}
	}
	if b.Shipping["CardTrader"] != 5 || b.Timing["regular"][SignalClimbing] == 0 {
		t.Errorf("defaults: %+v %+v", b.Shipping, b.Timing)
	}
}

func TestTimingFor(t *testing.T) {
	if f := timingFor("Collector Booster", SignalClimbing); math.Abs(f-1.12) > 1e-9 {
		t.Errorf("collector climbing: %v", f)
	}
	if f := timingFor("Play Booster", SignalNew); math.Abs(f-0.935) > 1e-9 {
		t.Errorf("new set: %v", f)
	}
	if f := timingFor("Play Booster", SignalPreorder); f != 1 {
		t.Errorf("no factor: %v", f)
	}
}

func TestBestDeals_NoCatalog(t *testing.T) {
	trendsDB(t)
	rr := httptest.NewRecorder()
	withMockClient(403, "", func() {
		newTestTrendsHandler(t).BestDeals(rr, httptest.NewRequest(http.MethodGet, "/api/best-deals", nil))
	})
	if rr.Code != http.StatusBadGateway {
		t.Errorf("no catalog: %d", rr.Code)
	}
}

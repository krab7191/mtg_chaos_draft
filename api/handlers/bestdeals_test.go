package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestBestDealsLive(t *testing.T) {
	ctx := ctDB(t)
	pool := testhelper.Pool(t)
	requests := fakeCardTrader(t, false)
	ctLiveMu.Lock()
	ctLiveSeen = map[int]ctLiveEntry{}
	ctLiveMu.Unlock()
	_ = db.SaveCardTraderExpansion(ctx, pool, 1, []db.CTBlueprint{{BlueprintID: 10, MTGStocksID: 1, Name: "Test Set Draft Booster"}})
	h := newTestTrendsHandler(t)
	get := func(q string) (int, []Deal) {
		rr := httptest.NewRecorder()
		withMockClient(200, fakeSetsJSON, func() { h.BestDealsLive(rr, httptest.NewRequest(http.MethodGet, "/api/best-deals/live"+q, nil)) })
		var resp struct{ Offers []Deal }
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		return rr.Code, resp.Offers
	}
	code, offers := get("?ids=1,4,x")
	if code != 200 || len(offers) != 2 {
		t.Fatalf("live: %d %+v", code, offers)
	}
	byLang := map[string]Deal{}
	for _, o := range offers {
		byLang[o.Language] = o
	}
	if byLang["en"].Price != 6.94 || byLang["ja"].Price != 4.95 || byLang["ja"].MarketPrice != 4.5 {
		t.Errorf("fresh prices: %+v", byLang)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("requests: %d", n)
	}
	// Within 5 minutes: cached, CardTrader isn't asked again.
	get("?ids=1")
	if n := requests.Load(); n != 1 {
		t.Errorf("cached: %d requests", n)
	}
	if code, _ := get("?ids="); code != http.StatusBadRequest {
		t.Errorf("no ids: %d", code)
	}
	// CardTrader down (after the cache expires): 502.
	ctLiveMu.Lock()
	ctLiveSeen = map[int]ctLiveEntry{}
	ctLiveMu.Unlock()
	cardTraderBase = "http://127.0.0.1:1"
	if code, _ := get("?ids=1"); code != http.StatusBadGateway {
		t.Errorf("CardTrader down: %d", code)
	}
	// No token: nothing to check.
	cardTraderToken = ""
	if code, offers := get("?ids=1"); code != 200 || len(offers) != 0 {
		t.Errorf("no token: %d %+v", code, offers)
	}
	// No catalog.
	rr := httptest.NewRecorder()
	resetProductsCache()
	withMockClient(403, "", func() { h.BestDealsLive(rr, httptest.NewRequest(http.MethodGet, "/api/best-deals/live?ids=1", nil)) })
	if rr.Code != http.StatusBadGateway {
		t.Errorf("no catalog: %d", rr.Code)
	}
}

func TestAlertLanguages_DealAlertsFollowThem(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "app_settings")
	if _, err := pool.Exec(ctx, `TRUNCATE app_settings`); err != nil {
		t.Fatal(err)
	}
	ntfy := newFakeNtfy(t)
	h := newTestTrendsHandler(t)
	call := func(method, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.AlertLanguages(rr, httptest.NewRequest(method, "/api/alert-languages", strings.NewReader(body)))
		return rr
	}
	if rr := call(http.MethodGet, ""); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"languages":null`) {
		t.Fatalf("unset: %d %s", rr.Code, rr.Body)
	}
	// A watched pack whose only deal is a French listing.
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 4})
	_ = db.ReplaceStoreOffers(ctx, pool, "CardTrader", []db.Offer{
		{URL: "fr", MTGStocksID: 4, Title: "Another Set Booster · French · ships from FR via Zero", Price: 2, Available: true, Language: "fr"},
	})
	if rr := call(http.MethodPut, `{"languages":["en","ja"]}`); rr.Code != http.StatusNoContent {
		t.Fatalf("put: %d", rr.Code)
	}
	if rr := call(http.MethodGet, ""); !strings.Contains(rr.Body.String(), `["en","ja"]`) {
		t.Errorf("get: %s", rr.Body)
	}
	withMockClient(200, fakeSetsJSON, func() {
		if n, err := NotifyAlerts(ctx, pool); n != 0 || err != nil {
			t.Errorf("French deal while French is off: %d %v (%v)", n, err, ntfy.bodies)
		}
	})
	// Every language again: the deal alerts.
	_ = call(http.MethodPut, `{"languages":null}`)
	withMockClient(200, fakeSetsJSON, func() {
		if n, err := NotifyAlerts(ctx, pool); n != 1 || err != nil {
			t.Errorf("French on: %d %v", n, err)
		}
	})
	if rr := call(http.MethodPut, `nope`); rr.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", rr.Code)
	}
	if _, err := alertLanguages(ctx, closedPool(t)); err == nil {
		t.Error("db down: want an error")
	}
	rr := httptest.NewRecorder()
	(&TrendsHandler{pool: closedPool(t)}).AlertLanguages(rr, httptest.NewRequest(http.MethodGet, "/api/alert-languages", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("get, db down: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	(&TrendsHandler{pool: closedPool(t)}).AlertLanguages(rr, httptest.NewRequest(http.MethodPut, "/api/alert-languages", strings.NewReader(`{"languages":[]}`)))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("put, db down: %d", rr.Code)
	}
}

func TestLiveCardKingdom(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		_, _ = w.Write([]byte(`{"meta":{"base_url":"https://ck.example/"},"data":[
			{"url":"mtg-sealed/another","name":"Another Set Booster Pack","price_retail":"2.49","qty_retail":3}]}`))
	}))
	defer srv.Close()
	orig := cardKingdomURL
	cardKingdomURL = srv.URL
	t.Cleanup(func() { cardKingdomURL = orig })
	ckLiveMu.Lock()
	ckLiveOffer = nil
	ckLiveMu.Unlock()
	c, _ := parseCatalog([]byte(fakeSetsJSON))
	idx := newPackIndex(c.products)
	got, err := liveCardKingdom(context.Background(), idx, []int{4, 1})
	if err != nil || len(got) != 1 || got[0].Price != 2.49 || got[0].SeenAt.IsZero() {
		t.Fatalf("live CK: %+v %v", got, err)
	}
	if got, _ := liveCardKingdom(context.Background(), idx, []int{1}); len(got) != 0 || n != 1 {
		t.Errorf("cached (one request), other pack: %+v, %d requests", got, n)
	}
	cardKingdomURL = ""
	if got, err := liveCardKingdom(context.Background(), idx, []int{4}); err != nil || len(got) != 0 {
		t.Errorf("off: %+v %v", got, err)
	}
	cardKingdomURL = "http://127.0.0.1:1"
	ckLiveMu.Lock()
	ckLiveOffer = nil
	ckLiveMu.Unlock()
	if _, err := liveCardKingdom(context.Background(), idx, []int{4}); err == nil {
		t.Error("unreachable: want an error")
	}
}

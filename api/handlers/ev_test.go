package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

// A tiny draft booster: one slot from a 2-card sheet (weights 3:1) and one
// foil slot with a single card.
const tstBoosters = `{"data":{"cards":[{"uuid":"card-a","name":"Alpha Card"},{"uuid":"card-b","name":"Beta Card"},{"uuid":"card-c","name":"Shiny Card"}],"booster":{
	"draft":{"boostersTotalWeight":1,"boosters":[{"weight":1,"contents":{"main":1,"shiny":1}}],
		"sheets":{"main":{"totalWeight":4,"cards":{"card-a":3,"card-b":1}},
		          "shiny":{"foil":true,"totalWeight":1,"cards":{"card-c":1}}}},
	"collector":{"boostersTotalWeight":1,"boosters":[{"weight":1,"contents":{"shiny":2}}],
		"sheets":{"shiny":{"foil":true,"totalWeight":1,"cards":{"card-c":1}}}},
	"jumpstart":{"boostersTotalWeight":1,"boosters":[{"weight":1,"contents":{"main":1}}],"sheets":{}}}}}`

// Prices: card-a $1 then $2 (day 2); card-b $4; card-c $10 foil (only etched
// for day 2: falls back to etched); an unwanted card is skipped.
const allPrices = `{"meta":{"date":"2026-11-02"},"data":{
	"card-a":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-11-01":1,"2026-11-02":2}}},"cardmarket":{"retail":{"normal":{"2026-11-01":9}}}}},
	"card-b":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-11-01":4}}}}},
	"card-c":{"paper":{"tcgplayer":{"retail":{"foil":{"2026-11-01":10},"etched":{"2026-11-02":12}}}}},
	"other":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-11-01":100}}}}}}}`

const todayPrices = `{"meta":{"date":"2026-11-03"},"data":{
	"card-a":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-11-03":3}}}}},
	"card-b":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-11-03":4}}}}},
	"card-c":{"paper":{"tcgplayer":{"retail":{"foil":{"2026-11-03":10}}}}}}}`

func gz(s string) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return b.Bytes()
}

// fakeMTGJSON serves pull rates for TST (ANS has none: 404) and prices.
func fakeMTGJSON(t *testing.T) *[]string {
	t.Helper()
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/TST.json.gz":
			_, _ = w.Write(gz(tstBoosters))
		case "/AllPrices.json.gz":
			_, _ = w.Write(gz(allPrices))
		case "/AllPricesToday.json.gz":
			_, _ = w.Write(gz(todayPrices))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	origBase, origDelay := mtgjsonBase, evFetchDelay
	mtgjsonBase, evFetchDelay = srv.URL, 0
	t.Cleanup(func() { mtgjsonBase, evFetchDelay = origBase, origDelay })
	return &got
}

func evDB(t *testing.T) context.Context {
	t.Helper()
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "mtgjson_boosters", "pack_ev")
	if _, err := pool.Exec(ctx, `TRUNCATE mtgjson_boosters, pack_ev`); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestPackEV_Math(t *testing.T) {
	var data struct {
		Data struct {
			Booster map[string]mjBooster `json:"booster"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(tstBoosters), &data); err != nil {
		t.Fatal(err)
	}
	prices := map[string]float64{"card-a|false": 1, "card-b|false": 4, "card-c|true": 10}
	lookup := func(uuid string, foil bool) (float64, bool) {
		k := uuid + "|false"
		if foil {
			k = uuid + "|true"
		}
		v, ok := prices[k]
		return v, ok
	}
	// main slot: 3/4 x $1 + 1/4 x $4 = $1.75; foil slot $10.
	if ev := packEV(data.Data.Booster["draft"], lookup); math.Abs(ev-11.75) > 1e-9 {
		t.Errorf("draft EV: %v", ev)
	}
	// A foil sheet card with only a non-foil price falls back to it.
	delete(prices, "card-c|true")
	prices["card-c|false"] = 2
	if ev := packEV(data.Data.Booster["collector"], lookup); math.Abs(ev-4) > 1e-9 {
		t.Errorf("collector fallback EV: %v", ev)
	}
}

func TestRefreshEV(t *testing.T) {
	ctx := evDB(t)
	pool := testhelper.Pool(t)
	requests := fakeMTGJSON(t)

	withMockClient(200, fakeSetsJSON, func() {
		if err := RefreshEV(ctx, pool); err != nil {
			t.Fatal(err)
		}
	})
	// First run: pull rates for both sets (ANS: none), then the 90-day file.
	if strings.Join(*requests, " ") != "/ANS.json.gz /TST.json.gz /AllPrices.json.gz" {
		t.Errorf("requests: %v", *requests)
	}
	evs, _ := db.LatestEV(ctx, pool, []int{1, 2, 4}, 1)
	// TST draft (id 1) on 11-02: main 3/4 x $2 + 1/4 x $4 = $2.50, foil etched $12 -> $14.50; 11-01: $11.75.
	if e := evs[1]; math.Abs(e.Value-14.5) > 0.001 || e.Before == nil || math.Abs(*e.Before-11.75) > 0.001 {
		t.Fatalf("draft EV: %+v", e)
	}
	if e := evs[2]; math.Abs(e.Value-24) > 0.001 { // collector: 2 x $12
		t.Errorf("collector EV: %+v", e)
	}
	if _, ok := evs[4]; ok {
		t.Error("a set without pull rates has no EV")
	}
	info := evInfo(evs[1])
	if info.Change == nil || math.Abs(*info.Change-(14.5/11.75-1)) > 1e-6 {
		t.Errorf("change: %+v", info)
	}

	// Two weeks of history now? No (2 days): it would read the 90-day file
	// again. Pretend it has 14 days, then the next run reads only today.
	for d := 3; d <= 16; d++ {
		_ = db.RecordEV(ctx, pool, map[time.Time]map[int]float64{day("2026-10-01").AddDate(0, 0, d): {99: 1}})
	}
	*requests = nil
	withMockClient(200, fakeSetsJSON, func() {
		if err := RefreshEV(ctx, pool); err != nil {
			t.Fatal(err)
		}
	})
	// Old sets aren't asked again; today's prices only.
	if strings.Join(*requests, " ") != "/AllPricesToday.json.gz" {
		t.Errorf("second run requests: %v", *requests)
	}
	if evs, _ := db.LatestEV(ctx, pool, []int{1}, 1); math.Abs(evs[1].Value-13.25) > 0.001 { // $3 x 3/4 + $4 x 1/4 + $10
		t.Errorf("today's EV: %+v", evs[1])
	}

	// The EV shows on table rows.
	h := NewTrendsHandler(pool)
	h.now = func() time.Time { return day("2026-11-03") }
	rr := httptest.NewRecorder()
	withMockClient(200, fakeSetsJSON, func() { h.Cheapest(rr, httptest.NewRequest(http.MethodGet, "/api/cheapest/trends", nil)) })
	if !strings.Contains(rr.Body.String(), `"ev":{"value":13.25`) {
		t.Errorf("rows: %s", rr.Body.String())
	}
}

func TestRefreshBoosters_NewSetsRecheckedWeekly(t *testing.T) {
	ctx := evDB(t)
	pool := testhelper.Pool(t)
	requests := fakeMTGJSON(t)
	targets := map[string]map[string]SearchResult{
		"tst": {"draft": {MTGStocksID: 1, ReleasedAt: "2026-11-01"}},
		"old": {"booster": {MTGStocksID: 5, ReleasedAt: "2001-01-01"}},
	}
	now := day("2026-11-03")
	if _, _, err := refreshBoosters(ctx, pool, targets, now); err != nil {
		t.Fatal(err)
	}
	*requests = nil
	// A week later: the new set (and the one with no data yet) are asked again.
	_, _ = pool.Exec(ctx, `UPDATE mtgjson_boosters SET fetched_at = NOW() - INTERVAL '8 days'`)
	b, _, err := refreshBoosters(ctx, pool, targets, time.Now().UTC())
	if err != nil || len(b["tst"]) != 2 {
		t.Fatalf("%v %v", b, err)
	}
	if len(*requests) != 2 {
		t.Errorf("weekly recheck: %v", *requests)
	}
	// A failing download stops asking for the day but keeps stored data.
	mtgjsonBase = "http://127.0.0.1:1"
	_, _ = pool.Exec(ctx, `UPDATE mtgjson_boosters SET fetched_at = NOW() - INTERVAL '8 days'`)
	if b, _, err := refreshBoosters(ctx, pool, targets, time.Now().UTC()); err != nil || len(b["tst"]) != 2 {
		t.Errorf("after a failure: %v %v", b, err)
	}
	if _, _, err := refreshBoosters(ctx, closedPool(t), targets, now); err == nil {
		t.Error("db down")
	}
}

func TestRefreshEV_Failures(t *testing.T) {
	ctx := evDB(t)
	pool := testhelper.Pool(t)
	fakeMTGJSON(t)
	withMockClient(403, "", func() {
		resetProductsCache()
		if err := RefreshEV(ctx, pool); err == nil {
			t.Error("no catalog: want an error")
		}
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/AllPrices") {
			_, _ = w.Write([]byte("not gzip"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	mtgjsonBase = srv.URL
	withMockClient(200, fakeSetsJSON, func() {
		resetProductsCache()
		if err := RefreshEV(ctx, pool); err == nil {
			t.Error("bad prices file: want an error")
		}
	})
	if _, err := readPrices(strings.NewReader(`{"meta":{}}`), nil); err == nil {
		t.Error("no data: want an error")
	}
	if _, _, err := fetchBoosters(ctx, "tst"); err == nil {
		t.Error("500: want an error")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/GZ.json.gz" {
			_, _ = w.Write([]byte("not gzip"))
			return
		}
		_, _ = w.Write(gz("not json"))
	}))
	defer bad.Close()
	mtgjsonBase = bad.URL
	for _, code := range []string{"gz", "js"} {
		if _, _, err := fetchBoosters(ctx, code); err == nil {
			t.Errorf("%s: want an error", code)
		}
	}
	mtgjsonBase = "::bad"
	if _, _, err := fetchBoosters(ctx, "x"); err == nil {
		t.Error("bad URL: want an error")
	}
	// Recording nothing is fine; a cancelled run stops.
	if err := db.RecordEV(ctx, pool, nil); err != nil {
		t.Error(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	targets := map[string]map[string]SearchResult{"a": {"draft": {}}, "b": {"draft": {}}}
	mtgjsonBase = srv.URL
	if _, _, err := refreshBoosters(cancelled, pool, targets, time.Now()); err == nil {
		t.Error("cancelled: want an error")
	}
}

func TestAssignBoosters_DraftAndPlainBoosterPacks(t *testing.T) {
	b := mjBooster{BoostersTotalWeight: 1}
	b.Boosters = append(b.Boosters, struct {
		Contents map[string]int `json:"contents"`
		Weight   float64        `json:"weight"`
	}{Weight: 1})
	targets := map[string]map[string]SearchResult{
		"old": {"booster": {MTGStocksID: 1}},
		"mid": {"draft": {MTGStocksID: 2}},
	}
	boosters := map[string]map[string]mjBooster{"old": {"draft": b}, "mid": {"default": b}}
	got := assignBoosters(targets, boosters)
	if len(got) != 2 || got[0].id != 1 || got[1].id != 2 {
		t.Errorf("assigned: %+v", got)
	}
}

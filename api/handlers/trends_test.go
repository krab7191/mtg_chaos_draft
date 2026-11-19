package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// series builds daily points from a start day and one price per day.
func series(start string, prices ...float64) []db.PricePoint {
	d := day(start)
	out := make([]db.PricePoint, len(prices))
	for i, p := range prices {
		out[i] = db.PricePoint{Day: d.AddDate(0, 0, i), Price: p}
	}
	return out
}

// repeat returns n copies of p.
func repeat(p float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = p
	}
	return out
}

// loadRealHistory reads a real MTGStocks /prices response from testdata.
func loadRealHistory(t *testing.T, file string) []db.PricePoint {
	t.Helper()
	f, err := os.Open("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	var points []db.PricePoint
	withMockClient(200, string(raw), func() {
		points, err = fetchHistory(1)
	})
	if err != nil {
		t.Fatal(err)
	}
	return points
}

// ── summarize / signals ──────────────────────────────────────

func TestSummarize_RealWildsOfEldraineSetBooster(t *testing.T) {
	points := loadRealHistory(t, "history_5382_woe_set_booster.json.gz")
	released := day("2023-09-08")
	tr := summarize(points, &released, day("2026-11-17"), defaultFloorRules["regular"])

	// Real history: bottomed at $4.58 two weeks after release (2023-09-22 UTC), now ~$16.70.
	if tr.Low != 4.58 || !tr.LowDay.Equal(day("2023-09-22")) {
		t.Errorf("low: %v on %v", tr.Low, tr.LowDay)
	}
	if tr.Current != 16.7 || tr.AboveLow < 2.5 {
		t.Errorf("current %v, above low %v", tr.Current, tr.AboveLow)
	}
	if tr.Signal == SignalFloor || tr.Signal == SignalFalling {
		t.Errorf("a set far above its low isn't a floor: %s", tr.Signal)
	}
	if len(tr.Spark) < 100 || tr.Spark[len(tr.Spark)-1] != 16.7 {
		t.Errorf("spark: %d points, last %v", len(tr.Spark), tr.Spark[len(tr.Spark)-1])
	}
	// Preorder prices (from May 2023) don't count toward the low.
	if tr.SparkStart.Before(released) {
		t.Errorf("spark starts before release: %v", tr.SparkStart)
	}
}

func TestSummarize_RealRealityFracturePlayBooster(t *testing.T) {
	points := loadRealHistory(t, "history_12545_rfa_play_booster.json.gz")
	released := day("2026-10-02")
	tr := summarize(points, &released, day("2026-11-17"), defaultFloorRules["regular"])
	if tr.Signal != SignalNew && tr.Signal != SignalFalling && tr.Signal != SignalFloor {
		t.Errorf("a six-week-old set at its low: got %s", tr.Signal)
	}
	if tr.AboveLow != 0 {
		t.Errorf("at its low today, got %v above", tr.AboveLow)
	}
}

func TestSignals(t *testing.T) {
	rel := day("2025-01-01")
	today := day("2026-01-01")
	hype := repeat(10, 20)
	slide := []float64{9.5, 9, 8.5, 8, 7.5, 7, 6.8, 6.6, 6.4, 6.2, 6.1, 6.0}
	wiggle := []float64{6.0, 6.1, 5.95, 6.05, 6.1, 5.98, 6.02, 6.08, 5.97, 6.03}

	cases := []struct {
		name   string
		prices []float64
		rel    *time.Time
		today  time.Time
		want   Signal
	}{
		{"preorder", repeat(10, 30), ptrTime(day("2026-02-01")), today, SignalPreorder},
		{"new set", repeat(10, 30), ptrTime(day("2025-12-15")), today, SignalNew},
		{"fell, then flat near the low", append(append(append(hype, slide...), repeat(6.0, 300)...), wiggle...), &rel, today, SignalFloor},
		{"still falling", append(append(hype, repeat(8, 300)...), slide...), &rel, today, SignalFalling},
		// Same shape as a floor, but only ~100 days after release: too early
		// (the backtest found lows a median ~6 months after release).
		{"too young for a floor", append(append(hype, slide...), repeat(6.0, 70)...), ptrTime(today.AddDate(0, 0, -100)), today, SignalStable},
		{"rising after a low", append(append(hype, repeat(6, 300)...), 6.5, 7, 7.5, 8), &rel, today, SignalRising},
		{"flat but never dropped much", append(hype, repeat(9.5, 320)...), &rel, today, SignalStable},
		{"no release date", repeat(5, 40), nil, today, SignalStable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := tc.today.AddDate(0, 0, -len(tc.prices)).Format("2006-01-02")
			tr := summarize(series(start, tc.prices...), tc.rel, tc.today, defaultFloorRules["regular"])
			if tr.Signal != tc.want {
				t.Errorf("want %s, got %s (drop %.2f, band %v, above low %.2f, change30 %v)",
					tc.want, tr.Signal, tr.DropFromRelease, deref(tr.Band), tr.AboveLow, deref(tr.Change30))
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func deref(f *float64) float64 {
	if f == nil {
		return -1
	}
	return *f
}

func TestSummarize_Edges(t *testing.T) {
	if summarize(nil, nil, day("2026-01-01"), defaultFloorRules["regular"]) != nil {
		t.Error("no history should give no trend")
	}
	// Short history: no 30-day change or band yet.
	tr := summarize(series("2025-12-25", 5, 5.1), nil, day("2026-01-01"), defaultFloorRules["regular"])
	if tr.Change30 != nil || tr.Band != nil {
		t.Errorf("short history: change %v band %v", tr.Change30, tr.Band)
	}
	// History that starts after release still finds a low and release price.
	rel := day("2020-01-01")
	tr = summarize(series("2025-06-01", 7, 6, 8), &rel, day("2026-01-01"), defaultFloorRules["regular"])
	if tr.Low != 6 || tr.ReleasePrice != 7 {
		t.Errorf("late history: low %v release %v", tr.Low, tr.ReleasePrice)
	}
}

func TestAlertsFor(t *testing.T) {
	drop := 0.3
	floor := &Trend{Signal: SignalFloor, DropFromRelease: 0.35}
	if got := alertsFor(floor, &db.Watch{DropAlert: &drop}, 0); strings.Join(got, ",") != "floor,drop" {
		t.Errorf("both: %v", got)
	}
	if got := alertsFor(&Trend{Signal: SignalFalling, DropFromRelease: 0.1}, &db.Watch{DropAlert: &drop}, 0); len(got) != 0 {
		t.Errorf("none: %v", got)
	}
	if got := alertsFor(floor, nil, 0); len(got) != 0 {
		t.Errorf("unwatched: %v", got)
	}
	if got := alertsFor(nil, &db.Watch{}, 0); len(got) != 0 {
		t.Errorf("no trend: %v", got)
	}
	// 20+ packs in the collection mute the alerts; 19 don't.
	if got := alertsFor(floor, &db.Watch{DropAlert: &drop}, 19); len(got) != 2 {
		t.Errorf("19 in stock: %v", got)
	}
	if got := alertsFor(floor, &db.Watch{DropAlert: &drop}, 20); len(got) != 0 {
		t.Errorf("20 in stock should mute: %v", got)
	}
}

// ── fetching and backfill ────────────────────────────────────

func TestFetchHistory(t *testing.T) {
	withMockClient(200, `{"market":[[1767225600000,5.5],[1767312000000,0]],"average":[[1767225600000,9]]}`, func() {
		p, err := fetchHistory(1)
		if err != nil || len(p) != 1 || p[0].Price != 5.5 || !p[0].Day.Equal(day("2026-01-01")) {
			t.Errorf("market series: %+v %v", p, err)
		}
	})
	withMockClient(200, `{"market":[],"average":[[1767225600000,9]]}`, func() {
		p, _ := fetchHistory(1)
		if len(p) != 1 || p[0].Price != 9 {
			t.Errorf("falls back to average: %+v", p)
		}
	})
	withMockClient(404, ``, func() {
		if p, err := fetchHistory(1); err != nil || p != nil {
			t.Errorf("404 means no history: %v %v", p, err)
		}
	})
	withMockClient(500, ``, func() {
		if _, err := fetchHistory(1); err == nil {
			t.Error("500 should fail")
		}
	})
	withMockClient(200, `nope`, func() {
		if _, err := fetchHistory(1); err == nil {
			t.Error("bad JSON should fail")
		}
	})
	withErrClient(func() {
		if _, err := fetchHistory(1); err == nil {
			t.Error("network error should fail")
		}
	})
}

// fakeMTGStocks serves the catalog and a history for every product.
type fakeMTGStocks struct{ history string }

func (f fakeMTGStocks) RoundTrip(r *http.Request) (*http.Response, error) {
	body := fakeSetsJSON
	switch {
	case strings.HasSuffix(r.URL.Path, "/prices"):
		body = f.history
	case strings.HasPrefix(r.URL.Path, "/sealed/"): // one product (its TCGplayer link)
		body = `{"id":1,"tcgUrl":"https://www.tcgplayer.com/product/5/pack"}`
	}
	return (&stubTransport{statusCode: 200, body: body}).RoundTrip(r)
}

func trendsDB(t *testing.T) context.Context {
	t.Helper()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "price_history", "price_history_backfill", "price_watches", "store_offers", "product_links")
	resetTCGLinks(t)
	resetProductsCache()
	t.Cleanup(resetProductsCache)
	return context.Background()
}

func TestBackfillPriceHistory(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	orig := backfillDelay
	backfillDelay = time.Millisecond
	t.Cleanup(func() { backfillDelay = orig })

	withTransport(fakeMTGStocks{history: `{"market":[[1767225600000,5.5]]}`}, func() {
		n, err := BackfillPriceHistory(ctx, pool)
		if err != nil || n != 3 {
			t.Fatalf("first run: %d %v", n, err)
		}
		// Everything is done now: a second run fetches nothing.
		if n, err := BackfillPriceHistory(ctx, pool); err != nil || n != 0 {
			t.Errorf("second run: %d %v", n, err)
		}
	})
	hist, _ := db.PriceHistory(ctx, pool, []int{1, 2, 4}, time.Time{})
	if len(hist) != 3 {
		t.Errorf("want history for 3 products, got %v", hist)
	}
}

func TestBackfillPriceHistory_StopsWhenMTGStocksFails(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	withMockClient(200, fakeSetsJSON, func() {
		if _, err := getCatalog(); err != nil {
			t.Fatal(err)
		}
	})
	withMockClient(403, "", func() {
		n, err := BackfillPriceHistory(ctx, pool)
		if err == nil || n != 0 {
			t.Errorf("want an error and nothing done, got %d %v", n, err)
		}
	})
	if done, _ := db.BackfilledIDs(ctx, pool); len(done) != 0 {
		t.Errorf("nothing should be marked done: %v", done)
	}
}

func TestBackfillPriceHistory_Cancelled(t *testing.T) {
	trendsDB(t)
	pool := testhelper.Pool(t)
	orig := backfillDelay
	backfillDelay = time.Hour
	t.Cleanup(func() { backfillDelay = orig })
	ctx, cancel := context.WithCancel(context.Background())
	withTransport(fakeMTGStocks{history: `{"market":[[1767225600000,5.5]]}`}, func() {
		go func() { time.Sleep(50 * time.Millisecond); cancel() }()
		n, err := BackfillPriceHistory(ctx, pool)
		if err == nil || n != 1 {
			t.Errorf("want 1 done then cancelled, got %d %v", n, err)
		}
	})
}

func TestBackfillPriceHistory_NoCatalog(t *testing.T) {
	trendsDB(t)
	withMockClient(403, "", func() {
		if _, err := BackfillPriceHistory(context.Background(), testhelper.Pool(t)); err == nil {
			t.Error("want an error without a catalog")
		}
	})
}

func TestRecordDailyPrices(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	c, _ := parseCatalog([]byte(fakeSetsJSON))
	c.fetchedAt, c.source = day("2026-11-18").Add(15*time.Hour), "upload"
	recordDailyPrices(ctx, pool, c) // uploaded snapshots aren't today's prices
	c.source = "live"
	recordDailyPrices(ctx, pool, c)
	recordDailyPrices(ctx, closedPool(t), c) // a failure is only logged

	hist, _ := db.PriceHistory(ctx, pool, []int{1, 2, 4}, time.Time{})
	if len(hist) != 3 || !hist[1][0].Day.Equal(day("2026-11-18")) {
		t.Errorf("recorded: %+v", hist)
	}
}

// ── API ──────────────────────────────────────────────────────

func newTestTrendsHandler(t *testing.T) *TrendsHandler {
	h := NewTrendsHandler(testhelper.Pool(t))
	h.now = func() time.Time { return day("2024-01-01") }
	return h
}

func TestTrendsList(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	// fakeSetsJSON: Test Set (2023-01-01) ids 1, 2; Another Set (2022-06-01) id 4.
	_ = db.StoreHistory(ctx, pool, 1, series("2023-01-01", append(repeat(10, 20), repeat(6, 300)...)...))
	drop := 0.3
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1, DropAlert: &drop})
	testhelper.Truncate(t, pool, "collection_packs")
	if _, err := db.AddPack(ctx, pool, 999, "x", "Test Set", "TST", "Draft Booster", 3, 1, nil, 15); err != nil {
		t.Fatal(err)
	}

	h := newTestTrendsHandler(t)
	get := func(url string) trendsResponse {
		rr := httptest.NewRecorder()
		withMockClient(200, fakeSetsJSON, func() { h.List(rr, httptest.NewRequest(http.MethodGet, url, nil)) })
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", url, rr.Code, rr.Body)
		}
		var resp trendsResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	resp := get("/api/trends?years=1") // only Test Set is within a year
	if len(resp.Products) != 2 || resp.Tracked != 3 || resp.Backfilled != 1 {
		t.Fatalf("products %d tracked %d backfilled %d", len(resp.Products), resp.Tracked, resp.Backfilled)
	}
	var watched *trendRow
	for i := range resp.Products {
		if resp.Products[i].MTGStocksID == 1 {
			watched = &resp.Products[i]
		}
	}
	if watched == nil || watched.Trend == nil || watched.Watch == nil || !strings.Contains(strings.Join(watched.Alerts, ","), AlertDrop) {
		t.Fatalf("watched row: %+v", watched)
	}
	if resp.Thresholds.FloorRules["collector"] != ruleFor("Collector Booster") || resp.Thresholds.FloorRules["regular"] != ruleFor("Play Booster") {
		t.Error("both floor rules should be reported")
	}

	if watched.Stock != 3 {
		t.Errorf("stock of the watched pack: want 3, got %d", watched.Stock)
	}
	if all := get("/api/trends?years=junk"); len(all.Products) != 3 {
		t.Errorf("default 2 years should include both sets, got %d", len(all.Products))
	}

	// A watched product older than the window still shows (it alerts the
	// phone too), with its full history: release price from release day.
	_ = db.StoreHistory(ctx, pool, 4, series("2022-06-01", append(repeat(10, 20), repeat(6, 500)...)...))
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 4, DropAlert: &drop})
	resp = get("/api/trends?years=1")
	var old *trendRow
	for i := range resp.Products {
		if resp.Products[i].MTGStocksID == 4 {
			old = &resp.Products[i]
		}
	}
	if len(resp.Products) != 3 || old == nil || old.Trend == nil || old.Trend.ReleasePrice != 10 || len(old.Alerts) == 0 {
		t.Fatalf("old watched row: %d products, %+v", len(resp.Products), old)
	}
}

func TestTrendsList_Errors(t *testing.T) {
	trendsDB(t)
	h := newTestTrendsHandler(t)
	rr := httptest.NewRecorder()
	withMockClient(403, "", func() { h.List(rr, httptest.NewRequest(http.MethodGet, "/api/trends", nil)) })
	if rr.Code != http.StatusBadGateway {
		t.Errorf("no catalog: want 502, got %d", rr.Code)
	}
}

func TestTrendsHistoryAndWatch(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	_ = db.StoreHistory(ctx, pool, 7, series("2026-01-01", 5, 6))
	h := newTestTrendsHandler(t)

	call := func(fn http.HandlerFunc, id, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		req.SetPathValue("mtgstocksId", id)
		rr := httptest.NewRecorder()
		fn(rr, req)
		return rr
	}

	if rr := call(h.History, "7", ""); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"price":6`) {
		t.Errorf("history: %d %s", rr.Code, rr.Body)
	}
	if rr := call(h.History, "8", ""); rr.Body.String() != "[]\n" {
		t.Errorf("empty history: %s", rr.Body)
	}
	if rr := call(h.History, "x", ""); rr.Code != 400 {
		t.Errorf("bad id: %d", rr.Code)
	}

	for body, want := range map[string]int{
		`{"watch":true,"dropAlert":0.25}`: 204,
		`{"watch":true}`:                  204,
		`{"watch":false}`:                 204,
		`{"watch":true,"dropAlert":1.5}`:  400,
		`{"watch":true,"dropAlert":0}`:    400,
		`nope`:                            400,
	} {
		if rr := call(h.SetWatch, "7", body); rr.Code != want {
			t.Errorf("%s: want %d, got %d", body, want, rr.Code)
		}
	}
	if rr := call(h.SetWatch, "x", `{}`); rr.Code != 400 {
		t.Errorf("bad id: %d", rr.Code)
	}

	closed := &TrendsHandler{pool: closedPool(t), now: time.Now}
	if rr := call(closed.History, "7", ""); rr.Code != 500 {
		t.Errorf("history db down: %d", rr.Code)
	}
	if rr := call(closed.SetWatch, "7", `{"watch":true}`); rr.Code != 500 {
		t.Errorf("watch db down: %d", rr.Code)
	}
	withMockClient(200, fakeSetsJSON, func() {
		rr := httptest.NewRecorder()
		closed.List(rr, httptest.NewRequest(http.MethodGet, "/api/trends", nil))
		if rr.Code != 500 {
			t.Errorf("list db down: %d", rr.Code)
		}
	})
}

func TestTrendsSetWatchMany(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	h := newTestTrendsHandler(t)
	put := func(hh *TrendsHandler, body string) int {
		rr := httptest.NewRecorder()
		hh.SetWatchMany(rr, httptest.NewRequest(http.MethodPut, "/api/trends/watch", strings.NewReader(body)))
		return rr.Code
	}
	if code := put(h, `{"ids":[1,2],"watch":true}`); code != 204 {
		t.Fatalf("watch: %d", code)
	}
	if w, _ := db.ListWatches(ctx, pool); len(w) != 2 {
		t.Errorf("watched: %v", w)
	}
	if code := put(h, `{"ids":[1],"watch":false}`); code != 204 {
		t.Fatalf("unwatch: %d", code)
	}
	if w, _ := db.ListWatches(ctx, pool); len(w) != 1 {
		t.Errorf("after unwatch: %v", w)
	}
	for _, bad := range []string{`nope`, `{"ids":[],"watch":true}`} {
		if code := put(h, bad); code != 400 {
			t.Errorf("%s: want 400, got %d", bad, code)
		}
	}
	if code := put(&TrendsHandler{pool: closedPool(t), now: time.Now}, `{"ids":[1],"watch":true}`); code != 500 {
		t.Errorf("db down: %d", code)
	}
}

func TestPackKindAndStock(t *testing.T) {
	for in, want := range map[string]string{
		"Play Booster Box": "play booster", "Play Booster": "play booster", "Sleeved Play Booster": "play booster",
		"Booster Box": "booster", "Booster Pack": "booster", "Collector Booster Box": "collector booster",
	} {
		if got := packKind(in); got != want {
			t.Errorf("%q: want %q, got %q", in, want, got)
		}
	}
	idx := newStockIndex([]db.CollectionPack{
		{SetName: "Bloomburrow", SetCode: "BLB", ProductType: "Play Booster", Quantity: 7},
		{SetName: "Bloomburrow", SetCode: "BLB", ProductType: "Sleeved Play Booster", Quantity: 1},
		{SetName: "Old Set", ProductType: "Booster Pack", Quantity: 4},
	})
	cases := []struct {
		p    SearchResult
		want int
	}{
		{SearchResult{SetName: "Bloomburrow", SetCode: "BLB", ProductType: "Play Booster Box"}, 8},
		{SearchResult{SetName: "Bloomburrow", SetCode: "blb", ProductType: "Play Booster"}, 8},
		{SearchResult{SetName: "Bloomburrow", SetCode: "BLB", ProductType: "Collector Booster Box"}, 0},
		{SearchResult{SetName: "Old Set", SetCode: "OLD", ProductType: "Booster Box"}, 4}, // matched by name
	}
	for _, tc := range cases {
		if got := idx.of(tc.p); got != tc.want {
			t.Errorf("%s %s: want %d, got %d", tc.p.SetName, tc.p.ProductType, tc.want, got)
		}
	}
}

func TestFloorRulesDifferForCollectorProducts(t *testing.T) {
	if ruleKind("Collector Booster Box") != "collector" || ruleKind("Collector Booster") != "collector" || ruleKind("Play Booster Box") != "regular" {
		t.Fatal("rule kinds")
	}
	// Released 200 days ago, slipped only ~5% from release, then steady near
	// a recent low for weeks: typical for a Collector box.
	today := day("2026-01-01")
	rel := today.AddDate(0, 0, -200)
	prices := append(append(repeat(100, 30), repeat(96, 140)...), repeat(95, 30)...)
	pts := series(rel.Format("2006-01-02"), prices...)
	if s := summarize(pts, &rel, today, defaultFloorRules["collector"]).Signal; s != SignalFloor {
		t.Errorf("collector rule: want floor, got %s", s)
	}
	if s := summarize(pts, &rel, today, defaultFloorRules["regular"]).Signal; s == SignalFloor {
		t.Errorf("regular rule needs a 10%% drop: got %s", s)
	}
}

func TestFloorNeedsARecentLow(t *testing.T) {
	// Fell to $6 early, recovered to $6.50 and sat there: the low (day 40) was
	// months ago, so this isn't a floor any more, it's a recovery.
	today := day("2026-01-01")
	rel := today.AddDate(0, 0, -300)
	prices := append(append(append(repeat(10, 20), repeat(8, 20)...), repeat(6, 10)...), repeat(6.2, 250)...)
	pts := series(rel.Format("2006-01-02"), prices...)
	stale := summarize(pts, &rel, today, defaultFloorRules["collector"])
	if stale.Signal == SignalFloor {
		t.Error("a low from months ago shouldn't read as a floor")
	}
	noRecency := defaultFloorRules["collector"]
	noRecency.Recent = 0
	if summarize(pts, &rel, today, noRecency).Signal != SignalFloor {
		t.Error("without the recency check it would (wrongly) be a floor")
	}
}

func TestLoadFloorRules_ReplacesOutdatedSeededDefaults(t *testing.T) {
	restoreRules(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "floor_rules")
	if _, err := pool.Exec(ctx, `TRUNCATE floor_rules`); err != nil {
		t.Fatal(err)
	}
	old := FloorRule{MinAge: 150, Drop: 0.10, Window: 14, Band: 0.12, Near: 0.10}
	raw, _ := json.Marshal(old)
	// An old seeded default is replaced; an approved rule is kept.
	_ = db.SetActiveRule(ctx, pool, "regular", raw, seededMarker)
	approved := FloorRule{MinAge: 120, Drop: 0.2, Window: 28, Band: 0.06, Near: 0.05, Recent: 30}
	rawApproved, _ := json.Marshal(approved)
	_ = db.SetActiveRule(ctx, pool, "collector", rawApproved, json.RawMessage(`{"proposed":{"hit5":0.3}}`))

	if err := LoadFloorRules(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if ruleFor("Play Booster") != defaultFloorRules["regular"] {
		t.Errorf("outdated default kept: %+v", ruleFor("Play Booster"))
	}
	if ruleFor("Collector Booster") != approved {
		t.Errorf("approved rule replaced: %+v", ruleFor("Collector Booster"))
	}
	active, _ := db.RulesByStatus(ctx, pool, "active")
	if len(active) != 2 {
		t.Errorf("active rows: %+v", active)
	}
}

// Older code seeded the defaults without any metrics; those are defaults
// too, so newer defaults replace them (this kept stale floors alive).
func TestLoadFloorRules_ReplacesDefaultsSeededWithoutMetrics(t *testing.T) {
	restoreRules(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "floor_rules")
	old := FloorRule{MinAge: 150, Drop: 0, Window: 14, Band: 0.08, Near: 0.20}
	raw, _ := json.Marshal(old)
	_ = db.SetActiveRule(ctx, pool, "collector", raw, nil)
	_ = db.SetActiveRule(ctx, pool, "regular", raw, json.RawMessage(`null`))
	if err := LoadFloorRules(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if ruleFor("Collector Booster") != defaultFloorRules["collector"] || ruleFor("Play Booster") != defaultFloorRules["regular"] {
		t.Errorf("old defaults kept: %+v / %+v", ruleFor("Collector Booster"), ruleFor("Play Booster"))
	}
	active, _ := db.RulesByStatus(ctx, pool, "active")
	if len(active) != 2 {
		t.Errorf("active rows: %+v", active)
	}
}

func TestNewLow(t *testing.T) {
	rel := day("2025-01-01")
	today := day("2026-01-01")
	// Settled around $6 for months, then broke clearly lower in the last week.
	broke := append(append(repeat(10, 20), repeat(6, 200)...), repeat(5.5, 10)...)
	tr := summarize(series("2025-01-01", broke...), &rel, today, defaultFloorRules["regular"])
	if !tr.NewLow || math.Abs(tr.PrevLow-6) > 1e-9 {
		t.Errorf("break below $6: newLow=%v prev=%v", tr.NewLow, tr.PrevLow)
	}
	if got := alertsFor(tr, &db.Watch{}, 0); !slices.Contains(got, AlertNewLow) {
		t.Errorf("alerts: %v", got)
	}
	// One cheap sale isn't a new low: the weekly average barely moves.
	blip := append(append(repeat(10, 20), repeat(6, 205)...), 5.0, 6, 6, 6, 6)
	if summarize(series("2025-01-01", blip...), &rel, today, defaultFloorRules["regular"]).NewLow {
		t.Error("a one-day dip shouldn't count")
	}
	// The post-release slide isn't a "new low" every day: too little history.
	slide := []float64{10, 9.8, 9.6, 9.4, 9.2, 9, 8.8, 8.6, 8.4, 8.2, 8, 7.8, 7.6, 7.4, 7.2, 7, 6.8, 6.6, 6.4, 6.2, 6, 5.8, 5.6}
	if summarize(series("2025-01-01", slide...), &rel, today, defaultFloorRules["regular"]).NewLow {
		t.Error("the release slide shouldn't count")
	}
	// Flat: no new low.
	if summarize(series("2025-01-01", repeat(6, 200)...), &rel, today, defaultFloorRules["regular"]).NewLow {
		t.Error("flat isn't a new low")
	}
}

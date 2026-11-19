package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

// restoreRules puts the default rules back after a test changes them.
func restoreRules(t *testing.T) {
	t.Cleanup(func() {
		for k, r := range defaultFloorRules {
			setRule(k, r)
		}
	})
}

// floorShape: $10 at release, sliding to $6 by day 60, flat until day 250,
// then climbing back: the low is $6, reached from day 60.
func floorShape(days int) []float64 {
	s := make([]float64, days)
	for i := range s {
		switch {
		case i < 60:
			s[i] = 10 - 4*float64(i)/60
		case i < 250:
			s[i] = 6
		default:
			s[i] = 6 + float64(i-250)/50
		}
	}
	return s
}

func TestBTSeries(t *testing.T) {
	rel := day("2024-01-01")
	pts := []db.PricePoint{{Day: day("2023-12-01"), Price: 50}} // preorder: ignored
	pts = append(pts, series("2024-01-05", floorShape(400)...)...)
	pts = pts[:len(pts)-1]
	// A gap: day 2024-01-07 missing is forward-filled.
	pts = append(pts[:3], pts[4:]...)
	offset, s, ok := btSeries(pts, rel)
	if !ok || offset != 4 || s[0] != 10 || len(s) != 399-1+1 {
		t.Fatalf("series: ok=%v offset=%d len=%d first=%v", ok, offset, len(s), s[0])
	}
	if s[2] != s[1] && s[2] == 0 {
		t.Errorf("gap not forward-filled: %v", s[:4])
	}
	// Capped at the horizon.
	_, long, _ := btSeries(series("2024-01-01", floorShape(900)...), rel)
	if len(long) != btHorizon+1 {
		t.Errorf("horizon: %d", len(long))
	}
	for name, p := range map[string][]db.PricePoint{
		"starts too late": series("2024-03-01", floorShape(400)...),
		"too short":       series("2024-01-01", floorShape(100)...),
		"only preorder":   series("2023-06-01", floorShape(100)...),
	} {
		if _, _, ok := btSeries(p, rel); ok {
			t.Errorf("%s: want rejected", name)
		}
	}
}

func TestReplayAndMetrics(t *testing.T) {
	p := newBTProduct("2024", 0, floorShape(540), []int{14, 28})
	rule := FloorRule{MinAge: 90, Drop: 0.2, Window: 14, Band: 0.05, Near: 0.05}
	o := replay(p, rule)
	if !o.fired || o.premium != 0 || o.false_ {
		t.Errorf("flat floor at the low: %+v", o)
	}
	// A rule demanding a 50% drop never fires on a 40% dip.
	if o := replay(p, FloorRule{MinAge: 90, Drop: 0.5, Window: 14, Band: 0.05, Near: 0.05}); o.fired {
		t.Error("should not fire")
	}
	// Firing early on the slide (no drop needed, wide band) buys above the
	// low and is a false floor: the price fell another 10%+.
	early := replay(p, FloorRule{MinAge: 0, Drop: 0, Window: 14, Band: 0.5, Near: 1})
	if !early.fired || early.premium <= 0 || !early.false_ {
		t.Errorf("early: %+v", early)
	}

	j := func(fired bool, premium float64, false_ bool) btOutcome {
		return btOutcome{judged: true, fired: fired, premium: premium, false_: false_}
	}
	m := metricsOf([]btOutcome{j(true, 0.03, false), j(true, 0.3, true), j(false, 0, false), j(true, 0.08, false), {} /* too young */})
	if m.Products != 4 || m.Coverage != 0.75 || math.Abs(m.Median-0.08) > 1e-9 || math.Abs(m.FalseRate-1.0/3) > 1e-9 ||
		math.Abs(m.Hit5-1.0/3) > 1e-9 || math.Abs(m.Hit10-2.0/3) > 1e-9 {
		t.Errorf("metrics: %+v", m)
	}
	// Hitting within 5% more often scores better (lower), even at a higher average.
	precise := BTMetrics{Coverage: 0.5, Hit5: 0.4, Mean: 0.15}
	loose := BTMetrics{Coverage: 0.9, Hit5: 0.1, Mean: 0.10}
	if precise.score() >= loose.score() {
		t.Errorf("score should favor hitting the bottom: %v vs %v", precise.score(), loose.score())
	}
	even := metricsOf([]btOutcome{j(true, 0.1, false), j(true, 0.3, false)})
	if math.Abs(even.Median-0.2) > 1e-9 {
		t.Errorf("even median: %v", even.Median)
	}
	if none := metricsOf([]btOutcome{{}, {}}); !math.IsInf(none.score(), 1) || none.Coverage != 0 {
		t.Errorf("never fires: %+v", none)
	}
	if empty := metricsOf(nil); empty.Coverage != 0 {
		t.Errorf("empty: %+v", empty)
	}
	// A product younger than a rule's minimum age (+30 days) isn't judged.
	young := newBTProduct("2026", 0, floorShape(150), []int{14, 28})
	if o := replay(young, FloorRule{MinAge: 150, Window: 14, Band: 1, Near: 1}); o.judged {
		t.Errorf("too young to judge: %+v", o)
	}
	// A stale low blocks the rule when recency is required.
	stale := append(append(repeat(10, 20), repeat(6, 10)...), repeat(6.2, 400)...)
	ps := newBTProduct("2024", 0, stale, []int{14, 28})
	if o := replay(ps, FloorRule{MinAge: 150, Window: 14, Band: 0.1, Near: 0.1, Recent: 60}); o.fired {
		t.Errorf("stale low fired: %+v", o)
	}
	if o := replay(ps, FloorRule{MinAge: 150, Window: 14, Band: 0.1, Near: 0.1}); !o.fired {
		t.Errorf("no recency check: should fire: %+v", o)
	}
}

func TestBestRuleAndCrossValidate(t *testing.T) {
	var products []btProduct
	for i := 0; i < 12; i++ {
		products = append(products, newBTProduct(fmt.Sprint(2021+i%4), 0, floorShape(540), []int{14, 28}))
	}
	never := FloorRule{MinAge: 500, Drop: 0.9, Window: 14, Band: 0.01, Near: 0.01}
	best := bestRule(products, never)
	if best == never {
		t.Fatal("a firing rule should beat one that never fires")
	}
	tuned, fixed := crossValidate(products, never)
	if tuned.Coverage < 0.9 || fixed.Coverage != 0 || tuned.score() >= fixed.score() {
		t.Errorf("tuned %+v vs fixed %+v", tuned, fixed)
	}
	// With nothing that qualifies, the fallback stands.
	short := []btProduct{newBTProduct("2024", 0, repeat(5, 30), []int{14, 28})}
	if r := bestRule(short, never); r != never {
		t.Errorf("fallback: %+v", r)
	}
}

// backtestFixture: a catalog of n sets (released 2021–2024) of one pack type
// each, with a floor-shaped history in the database, backfilled.
func backtestFixture(t *testing.T, n int, productType string) context.Context {
	t.Helper()
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "floor_rules", "backtest_runs")
	if _, err := pool.Exec(ctx, `TRUNCATE floor_rules, backtest_runs`); err != nil {
		t.Fatal(err)
	}
	var sets []string
	for i := 0; i < n; i++ {
		rel := day("2021-01-01").AddDate(0, 0, i*40)
		sets = append(sets, fmt.Sprintf(`{"name":"Set %d","abbreviation":"S%d","date":%q,"products":[{"id":%d,"name":%q,"type":null,"latestPrice":{"market":6}}]}`,
			i, i, rel.Format("2006-01-02"), 1000+i, productType))
		if err := db.StoreHistory(ctx, pool, 1000+i, series(rel.Format("2006-01-02"), floorShape(540)...)); err != nil {
			t.Fatal(err)
		}
	}
	c, err := parseCatalog([]byte("[" + strings.Join(sets, ",") + "]"))
	if err != nil {
		t.Fatal(err)
	}
	c.fetchedAt, c.source = time.Now(), "live"
	catalogMu.Lock()
	currentCatalog, lastLiveAttempt = c, time.Now()
	catalogMu.Unlock()
	return ctx
}

func TestRunBacktest_ProposesAClearlyBetterRule(t *testing.T) {
	restoreRules(t)
	ctx := backtestFixture(t, 32, "Play Booster")
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	// A regular rule that never fires: anything that does should be proposed.
	setRule("regular", FloorRule{MinAge: 500, Drop: 0.9, Window: 14, Band: 0.01, Near: 0.01})

	summary, err := RunBacktest(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	reg := summary.Kinds["regular"]
	if reg.Products != 32 || !reg.Proposed || reg.Best == nil {
		t.Fatalf("regular: %+v", reg)
	}
	if col := summary.Kinds["collector"]; col.Proposed || !strings.Contains(col.Note, "only 0 products") {
		t.Errorf("collector with no data: %+v", col)
	}
	if ntfy.count() != 1 || ntfy.got[0].Get("Title") != "New floor rule proposed" || !strings.Contains(ntfy.bodies[0], "regular:") {
		t.Errorf("notification: %d %v", ntfy.count(), ntfy.bodies)
	}
	// The active rule is unchanged until approved.
	if ruleFor("Play Booster").MinAge != 500 {
		t.Error("a proposal must not change the active rule")
	}
	if at, _, _ := db.LastBacktest(ctx, pool); at.IsZero() {
		t.Error("run not recorded")
	}

	// It shows on the trends page...
	h := newTestTrendsHandler(t)
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/trends?years=10", nil))
	var resp trendsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Proposals) != 1 || resp.Proposals[0].Kind != "regular" || resp.Proposals[0].Current.MinAge != 500 || resp.LastBacktest == nil {
		t.Fatalf("proposals on page: %+v %v", resp.Proposals, resp.LastBacktest)
	}

	// ...and applying it takes effect at once.
	call := func(fn http.HandlerFunc, id string) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		fn(rr, req)
		return rr.Code
	}
	id := fmt.Sprint(resp.Proposals[0].ID)
	if code := call(h.ApplyRule, id); code != 204 {
		t.Fatalf("apply: %d", code)
	}
	if ruleFor("Play Booster") != resp.Proposals[0].Rule {
		t.Errorf("active rule after apply: %+v", ruleFor("Play Booster"))
	}
	if code := call(h.ApplyRule, id); code != 409 {
		t.Errorf("applying twice: %d", code)
	}
	if code := call(h.DismissRule, "x"); code != 400 {
		t.Errorf("bad id: %d", code)
	}
	closed := &TrendsHandler{pool: closedPool(t), now: time.Now}
	if code := call(closed.DismissRule, "1"); code != 500 {
		t.Errorf("db down: %d", code)
	}

	// Rerunning now proposes nothing: the applied rule is the best.
	again, err := RunBacktest(ctx, pool)
	if err != nil || again.Kinds["regular"].Proposed {
		t.Errorf("second run: %+v %v", again.Kinds["regular"], err)
	}
}

func TestDismissRule(t *testing.T) {
	restoreRules(t)
	ctx := backtestFixture(t, 32, "Play Booster")
	pool := testhelper.Pool(t)
	setRule("regular", FloorRule{MinAge: 500, Drop: 0.9, Window: 14, Band: 0.01, Near: 0.01})
	if _, err := RunBacktest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	open, _ := openProposals(ctx, pool)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", fmt.Sprint(open[0].ID))
	rr := httptest.NewRecorder()
	newTestTrendsHandler(t).DismissRule(rr, req)
	if rr.Code != 204 || ruleFor("Play Booster").MinAge != 500 {
		t.Errorf("dismiss: %d %+v", rr.Code, ruleFor("Play Booster"))
	}
	if open, _ := openProposals(ctx, pool); len(open) != 0 {
		t.Errorf("still open: %+v", open)
	}
}

func TestBacktestEndpoint(t *testing.T) {
	restoreRules(t)
	backtestFixture(t, 5, "Play Booster")
	h := newTestTrendsHandler(t)
	rr := httptest.NewRecorder()
	h.Backtest(rr, httptest.NewRequest(http.MethodPost, "/api/trends/backtest", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "only 5 products") {
		t.Errorf("run now: %d %s", rr.Code, rr.Body)
	}
	resetProductsCache()
	rr = httptest.NewRecorder()
	withMockClient(403, "", func() { h.Backtest(rr, httptest.NewRequest(http.MethodPost, "/", nil)) })
	if rr.Code != 502 {
		t.Errorf("no catalog: %d", rr.Code)
	}
}

func TestMaybeRunBacktest(t *testing.T) {
	restoreRules(t)
	ctx := backtestFixture(t, 3, "Play Booster")
	pool := testhelper.Pool(t)
	// Backfill incomplete (fixture products aren't marked done): waits.
	if _, err := pool.Exec(ctx, `TRUNCATE price_history_backfill`); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunBacktest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if at, _, _ := db.LastBacktest(ctx, pool); !at.IsZero() {
		t.Fatal("ran before the backfill finished")
	}
	// Backfilled: runs once...
	for i := 0; i < 3; i++ {
		_ = db.StoreHistory(ctx, pool, 1000+i, nil)
	}
	if err := MaybeRunBacktest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	first, _, _ := db.LastBacktest(ctx, pool)
	if first.IsZero() {
		t.Fatal("should have run")
	}
	// ...and not again within the quarter.
	if err := MaybeRunBacktest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var runs int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM backtest_runs`).Scan(&runs)
	if runs != 1 {
		t.Errorf("want 1 run, got %d", runs)
	}
	if err := MaybeRunBacktest(ctx, closedPool(t)); err == nil {
		t.Error("db down should be an error")
	}
}

func TestLoadFloorRules(t *testing.T) {
	restoreRules(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "floor_rules")
	if _, err := pool.Exec(ctx, `TRUNCATE floor_rules`); err != nil {
		t.Fatal(err)
	}
	// Empty: seeds the defaults.
	if err := LoadFloorRules(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if a, _ := db.RulesByStatus(ctx, pool, "active"); len(a) != 2 {
		t.Fatalf("seeded: %+v", a)
	}
	// A stored active rule wins over the default.
	custom := FloorRule{MinAge: 120, Drop: 0.2, Window: 28, Band: 0.06, Near: 0.05}
	raw, _ := json.Marshal(custom)
	_ = db.SetActiveRule(ctx, pool, "regular", raw, nil)
	setRule("regular", defaultFloorRules["regular"])
	if err := LoadFloorRules(ctx, pool); err != nil || ruleFor("Play Booster") != custom {
		t.Errorf("loaded: %+v %v", ruleFor("Play Booster"), err)
	}
	// Corrupt JSON is reported.
	_ = db.SetActiveRule(ctx, pool, "collector", []byte(`{"minAge":"x"}`), nil)
	if err := LoadFloorRules(ctx, pool); err == nil {
		t.Error("bad rule JSON should fail")
	}
	if err := LoadFloorRules(ctx, closedPool(t)); err == nil {
		t.Error("db down should fail")
	}
}

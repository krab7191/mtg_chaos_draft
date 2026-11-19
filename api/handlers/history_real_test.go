package handlers

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/db"
)

// Real price histories (MTGStocks market prices, 265 packs and boxes, up to
// 2026-11-18) run through the signals and the backtest. The golden file
// records each product's signal month by month; any change to the rules or
// the code shows up there as a diff to review, product by product.
//
// Regenerate with: go test ./handlers -run TestRealHistories -update

const realSignalsGolden = "testdata/real_signals.golden.txt"

type realHistory struct {
	ID       int        `json:"id"`
	Set      string     `json:"set"`
	Type     string     `json:"type"`
	Released string     `json:"released"`
	First    string     `json:"first"`
	Prices   []*float64 `json:"prices"` // daily from First; null = no price that day
}

func loadRealHistories(t *testing.T) []realHistory {
	t.Helper()
	f, err := os.Open("testdata/real_histories.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var out []realHistory
	if err := json.NewDecoder(zr).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (h realHistory) points() []db.PricePoint {
	first := day(h.First)
	var pts []db.PricePoint
	for i, p := range h.Prices {
		if p != nil {
			pts = append(pts, db.PricePoint{Day: first.AddDate(0, 0, i), Price: *p})
		}
	}
	return pts
}

var signalLetter = map[Signal]string{
	SignalPreorder: "p", SignalNew: "n", SignalFloor: "F", SignalFalling: "d", SignalRising: "r", SignalStable: "s",
}

func TestRealHistories(t *testing.T) {
	hs := loadRealHistories(t)
	if len(hs) < 250 {
		t.Fatalf("only %d histories", len(hs))
	}
	var b strings.Builder
	b.WriteString("# Signal every 30 days from the first price, as of that day (history up to it only).\n")
	b.WriteString("# p preorder, n new, F possible floor, d falling, r rising, s stable; ! = new low.\n")
	byKind := map[string][]btProduct{}
	for _, h := range hs {
		if isJumpstart(h.Set, h.Type) {
			continue // not tracked
		}
		pts := h.points()
		rel := day(h.Released)
		rule := defaultFloorRules[ruleKind(h.Type)]
		var line strings.Builder
		for end := 30; end <= len(pts); end += 30 {
			upto := pts[:end]
			tr := summarize(upto, &rel, upto[len(upto)-1].Day, rule)
			line.WriteString(signalLetter[tr.Signal])
			if tr.NewLow {
				line.WriteString("!")
			}
		}
		fmt.Fprintf(&b, "%s | %s | %s\n", h.Set, h.Type, line.String())
		if offset, s, ok := btSeries(pts, rel); ok && rel.Year() >= 2019 {
			byKind[ruleKind(h.Type)] = append(byKind[ruleKind(h.Type)], newBTProduct(h.Released[:4], offset, s, []int{14, 28}))
		}
	}
	b.WriteString("\n# Backtest of the default rules over these histories.\n")
	for _, kind := range []string{"regular", "collector"} {
		m := metricsOf(evaluateRule(byKind[kind], defaultFloorRules[kind]))
		fmt.Fprintf(&b, "%s: judged %d, fires %.0f%%, within 5%% %.0f%%, within 10%% %.0f%%, median %.1f%% above the low, false floors %.0f%%\n",
			kind, m.Products, m.Coverage*100, m.Hit5*100, m.Hit10*100, m.Median*100, m.FalseRate*100)
	}

	got := b.String()
	if *updateGolden {
		if err := os.WriteFile(realSignalsGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(realSignalsGolden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gl) && i < len(wl); i++ {
			if gl[i] != wl[i] {
				t.Errorf("first difference on line %d:\n got: %s\nwant: %s", i+1, gl[i], wl[i])
				break
			}
		}
		t.Fatal("signals on real histories changed; review, then run with -update")
	}
}

// Cases the user checked by eye.
func TestRealHistories_KnownCases(t *testing.T) {
	find := func(set, typ string) realHistory {
		for _, h := range loadRealHistories(t) {
			if h.Set == set && h.Type == typ {
				return h
			}
		}
		t.Fatalf("no history for %s %s", set, typ)
		return realHistory{}
	}
	// Secrets of Strixhaven Collector Box: low in June, flat to rising since.
	// Not a floor (it was, under the old rule with no recency check).
	h := find("Secrets of Strixhaven", "Collector Booster Box")
	pts := h.points()
	rel := day(h.Released)
	tr := summarize(pts, &rel, pts[len(pts)-1].Day, defaultFloorRules["collector"])
	if tr.Signal == SignalFloor {
		t.Errorf("Strixhaven Collector Box flagged as a floor: low %v on %v, now %v", tr.Low, tr.LowDay, tr.Current)
	}
	old := defaultFloorRules["collector"]
	old.Recent = 0
	if summarize(pts, &rel, pts[len(pts)-1].Day, old).Signal != SignalFloor {
		t.Error("expected the old rule to (wrongly) call it a floor; the fixture may have changed")
	}

	// Preorder prices sliding toward release are never a "new low".
	for _, h := range loadRealHistories(t) {
		rel := day(h.Released)
		pts := h.points()
		for end := 30; end <= len(pts); end += 30 {
			upto := pts[:end]
			asOf := upto[len(upto)-1].Day
			if tr := summarize(upto, &rel, asOf, defaultFloorRules[ruleKind(h.Type)]); tr.NewLow && rel.After(asOf) {
				t.Fatalf("%s %s: new low during preorder on %s", h.Set, h.Type, asOf.Format(time.DateOnly))
			}
		}
	}
}

// Jumpstart and sleeved packs aren't tracked; draft packs and boxes are.
func TestTrackedProducts_RealCatalog(t *testing.T) {
	c, _ := loadRealCatalog(t)
	tracked := trackedProducts(c)
	boxes, jumpstartInCatalog := 0, 0
	for _, p := range append(c.products, c.boxes...) {
		if isJumpstart(p.SetName, p.ProductType) {
			jumpstartInCatalog++
		}
	}
	for _, p := range tracked {
		if isJumpstart(p.SetName, p.ProductType) || isSleeved(p) {
			t.Errorf("tracked: %s %s", p.SetName, p.ProductType)
		}
		if p.PacksPerBox > 0 {
			boxes++
		}
	}
	if jumpstartInCatalog < 15 || boxes < 50 || len(tracked) < 200 {
		t.Errorf("jumpstart in catalog %d, tracked %d (%d boxes)", jumpstartInCatalog, len(tracked), boxes)
	}
}

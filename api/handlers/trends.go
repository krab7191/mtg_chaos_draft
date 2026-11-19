package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Price trends: daily prices for single packs and booster boxes, recorded
// every day and backfilled from MTGStocks' history, to show where each set
// is between release hype, the post-release low, and scarcity.

// ── Recording ─────────────────────────────────────────────────

// trackedProducts are the products whose prices are recorded: single packs
// (one per set and pack type, not sleeved) and booster boxes.
func trackedProducts(c *catalog) []SearchResult {
	out := make([]SearchResult, 0, len(c.products)+len(c.boxes))
	for _, p := range c.products {
		if !isSleeved(p) { // can't be hunted down after release weeks
			out = append(out, p)
		}
	}
	return append(out, c.boxes...)
}

// recordDailyPrices stores today's prices from a live catalog. Saved or
// uploaded catalogs are skipped: their prices aren't today's.
func recordDailyPrices(ctx context.Context, pool *pgxpool.Pool, c *catalog) {
	if c.source != "live" {
		return
	}
	prices := make(map[int]float64)
	for _, p := range trackedProducts(c) {
		if p.MarketPrice != nil {
			prices[p.MTGStocksID] = *p.MarketPrice
		}
	}
	day := c.fetchedAt.UTC().Truncate(24 * time.Hour)
	if err := db.RecordPrices(ctx, pool, day, prices); err != nil {
		log.Printf("price history: recording %d prices: %v", len(prices), err)
		return
	}
	log.Printf("price history: recorded %d prices for %s", len(prices), day.Format("2006-01-02"))
}

// ── Backfill ──────────────────────────────────────────────────

// backfillDelay spaces out history requests so a backfill is gentle on
// MTGStocks, which rate-limits fast clients. A 429 stops the run; it resumes
// on the next hourly run.
var backfillDelay = 5 * time.Second

// fetchHistory gets a product's daily market prices from MTGStocks, falling
// back to the average series when there is no market series.
func fetchHistory(mtgstocksID int) ([]db.PricePoint, error) {
	body, status, err := proxyMTGStocks(fmt.Sprintf("https://api.mtgstocks.com/sealed/%d/prices", mtgstocksID))
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil // no history: mark done anyway
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("mtgstocks returned %d", status)
	}
	var series map[string][][2]float64
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	raw := series["market"]
	if len(raw) == 0 {
		raw = series["average"]
	}
	points := make([]db.PricePoint, 0, len(raw))
	for _, pt := range raw {
		if pt[1] > 0 {
			day := time.UnixMilli(int64(pt[0])).UTC().Truncate(24 * time.Hour)
			points = append(points, db.PricePoint{Day: day, Price: pt[1]})
		}
	}
	return points, nil
}

// BackfillPriceHistory fetches the full history of every tracked product not
// yet backfilled. It stops early, to resume next time, when ctx ends or
// MTGStocks fails (blocked, or the proxy tunnel is off). Returns how many
// products were backfilled.
func BackfillPriceHistory(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	c, err := getCatalog()
	if err != nil {
		return 0, err
	}
	done, err := db.BackfilledIDs(ctx, pool)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range trackedProducts(c) {
		if done[p.MTGStocksID] {
			continue
		}
		if n > 0 {
			select {
			case <-time.After(backfillDelay):
			case <-ctx.Done():
				return n, ctx.Err()
			}
		}
		points, err := fetchHistory(p.MTGStocksID)
		if err != nil {
			return n, fmt.Errorf("history for %s %s: %w", p.SetName, p.ProductType, err)
		}
		if err := db.StoreHistory(ctx, pool, p.MTGStocksID, points); err != nil {
			return n, err
		}
		done[p.MTGStocksID] = true
		n++
	}
	if n > 0 {
		log.Printf("price history: backfilled %d products", n)
	}
	return n, nil
}

// ── Signals ───────────────────────────────────────────────────

// Signal thresholds, shown on the page so the labels can be checked.
const (
	newSetDays  = 45   // first weeks after release: hype prices
	trendWindow = 30   // days the trend is measured over
	trendMove   = 0.05 // ±5% over the window counts as falling/rising

)

// FloorRule says when a product counts as a possible floor: old enough,
// down from release, then within a narrow band for a while, near its low.
type FloorRule struct {
	MinAge int     `json:"minAge"` // days since release
	Drop   float64 `json:"drop"`   // at least this far below the release price
	Window int     `json:"window"` // days the band is measured over
	Band   float64 `json:"band"`   // highest/lowest in the window within this
	Near   float64 `json:"near"`   // and within this of the low since release
}

// floorRules are tuned by backtesting on real price histories (sets 2019 to
// mid 2025), with leave-one-release-year-out cross-validation:
//   - regular packs and boxes (102): fires on ~79%, buying ~10% (median)
//     above the eventual low, against ~17% for "wait 3 months";
//   - Collector packs and boxes (61) get their own rule: their prices often
//     don't drop much, so no minimum drop and a wider "near the low"; fires
//     on ~79% at ~16% above the low, against ~22% for "wait 3 months".
var floorRules = map[string]FloorRule{
	"regular":   {MinAge: 150, Drop: 0.10, Window: 14, Band: 0.12, Near: 0.10},
	"collector": {MinAge: 150, Drop: 0.00, Window: 14, Band: 0.08, Near: 0.20},
}

// ruleKind is which floor rule applies to a product type.
func ruleKind(productType string) string {
	if strings.Contains(strings.ToLower(productType), "collector") {
		return "collector"
	}
	return "regular"
}

// Signal is where a product is in its price cycle.
type Signal string

const (
	SignalPreorder Signal = "preorder" // not released yet
	SignalNew      Signal = "new"      // released under newSetDays ago
	SignalFalling  Signal = "falling"  // down over trendMove in trendWindow
	SignalFloor    Signal = "floor"    // fell from release, then flat near its low
	SignalRising   Signal = "rising"   // up over trendMove in trendWindow
	SignalStable   Signal = "stable"   // flat, well above its low
)

// Trend summarizes a product's price history.
type Trend struct {
	Current  float64   `json:"current"`
	Low      float64   `json:"low"`
	LowDay   time.Time `json:"lowDay"`
	AboveLow float64   `json:"aboveLow"` // fraction above the low
	Change30 *float64  `json:"change30"` // fraction over trendWindow; nil without enough history
	// ReleasePrice is the first price on or after release; DropFromRelease
	// how far below it the price is now (negative when above).
	ReleasePrice    float64 `json:"releasePrice"`
	DropFromRelease float64 `json:"dropFromRelease"`
	// Band is the spread between the highest and lowest price over the
	// rule's window, as a fraction of the lowest; nil without the history.
	Band        *float64   `json:"band"`
	Signal      Signal     `json:"signal"`
	Spark       []float64  `json:"spark"` // weekly prices since release, for a sparkline
	SparkStart  time.Time  `json:"sparkStart"`
	HistoryDays int        `json:"historyDays"`
	Released    *time.Time `json:"released"`
}

// priceOn returns the last price on or before day.
func priceOn(points []db.PricePoint, day time.Time) (float64, bool) {
	i := sort.Search(len(points), func(i int) bool { return points[i].Day.After(day) })
	if i == 0 {
		return 0, false
	}
	return points[i-1].Price, true
}

// summarize computes a product's trend from its daily prices (oldest first)
// as of today. Prices before release (preorders) are ignored for the low.
func summarize(points []db.PricePoint, released *time.Time, today time.Time, rule FloorRule) *Trend {
	if len(points) == 0 {
		return nil
	}
	last := points[len(points)-1]
	t := &Trend{Current: last.Price, HistoryDays: len(points), Released: released}

	since := points
	if released != nil {
		i := sort.Search(len(points), func(i int) bool { return !points[i].Day.Before(*released) })
		if i < len(points) {
			since = points[i:]
		}
	}
	t.Low, t.LowDay = since[0].Price, since[0].Day
	for _, p := range since {
		if p.Price < t.Low {
			t.Low, t.LowDay = p.Price, p.Day
		}
	}
	t.AboveLow = t.Current/t.Low - 1

	if before, ok := priceOn(points, last.Day.AddDate(0, 0, -trendWindow)); ok && before > 0 {
		change := t.Current/before - 1
		t.Change30 = &change
	}

	t.ReleasePrice = since[0].Price
	t.DropFromRelease = 1 - t.Current/t.ReleasePrice

	// Band over the rule's window, if history covers it.
	windowStart := last.Day.AddDate(0, 0, -rule.Window)
	if !points[0].Day.After(windowStart) {
		lo, hi := math.Inf(1), math.Inf(-1)
		for i := len(points) - 1; i >= 0 && !points[i].Day.Before(windowStart); i-- {
			lo, hi = math.Min(lo, points[i].Price), math.Max(hi, points[i].Price)
		}
		band := hi/lo - 1
		t.Band = &band
	}

	// Weekly sparkline since release (or since history starts).
	t.SparkStart = since[0].Day
	for d := since[0].Day; !d.After(last.Day); d = d.AddDate(0, 0, 7) {
		if p, ok := priceOn(points, d); ok {
			t.Spark = append(t.Spark, math.Round(p*100)/100)
		}
	}
	if len(t.Spark) == 0 || t.Spark[len(t.Spark)-1] != last.Price {
		t.Spark = append(t.Spark, last.Price)
	}

	t.Signal = signalFor(t, released, today, rule)
	return t
}

func signalFor(t *Trend, released *time.Time, today time.Time, rule FloorRule) Signal {
	switch {
	case released != nil && released.After(today):
		return SignalPreorder
	case released != nil && today.Sub(*released) < newSetDays*24*time.Hour:
		return SignalNew
	case isFloor(t, released, today, rule):
		return SignalFloor
	case t.Change30 != nil && *t.Change30 <= -trendMove:
		return SignalFalling
	case t.Change30 != nil && *t.Change30 >= trendMove:
		return SignalRising
	default:
		return SignalStable
	}
}

// isFloor: old enough, down from release, then flat for the rule's window
// near the low since release.
func isFloor(t *Trend, released *time.Time, today time.Time, rule FloorRule) bool {
	oldEnough := released != nil && today.Sub(*released) >= time.Duration(rule.MinAge)*24*time.Hour
	return oldEnough && t.DropFromRelease >= rule.Drop && t.Band != nil && *t.Band <= rule.Band && t.AboveLow <= rule.Near
}

// ── API ───────────────────────────────────────────────────────

// TrendsHandler serves the price trends page.
type TrendsHandler struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewTrendsHandler(pool *pgxpool.Pool) *TrendsHandler {
	return &TrendsHandler{pool: pool, now: time.Now}
}

type trendRow struct {
	SearchResult
	Trend  *Trend    `json:"trend"`
	Watch  *db.Watch `json:"watch"`
	Alerts []string  `json:"alerts"`
	// Stock is how many packs of this set and pack type are in the
	// collection (a box row counts loose packs of the kind it holds).
	Stock int `json:"stock"`
}

// packKind normalizes a product type to the pack it is or holds, so boxes,
// loose packs and sleeved packs of one kind match: "Play Booster Box",
// "Play Booster" and "Sleeved Play Booster" → "play booster".
func packKind(productType string) string {
	k := strings.ToLower(strings.TrimSpace(productType))
	k = strings.TrimPrefix(k, "sleeved ")
	for _, suffix := range []string{" box", " pack"} {
		k = strings.TrimSuffix(k, suffix)
	}
	return k
}

// stockIndex counts collection packs by set (code and name) and pack kind.
type stockIndex map[string]int

func newStockIndex(packs []db.CollectionPack) stockIndex {
	idx := stockIndex{}
	for _, p := range packs {
		kind := packKind(p.ProductType)
		if p.SetCode != "" {
			idx["code|"+strings.ToLower(p.SetCode)+"|"+kind] += p.Quantity
		}
		idx["name|"+strings.ToLower(p.SetName)+"|"+kind] += p.Quantity
	}
	return idx
}

func (idx stockIndex) of(p SearchResult) int {
	kind := packKind(p.ProductType)
	if p.SetCode != "" {
		if n, ok := idx["code|"+strings.ToLower(p.SetCode)+"|"+kind]; ok {
			return n
		}
	}
	return idx["name|"+strings.ToLower(p.SetName)+"|"+kind]
}

// Alert reasons for watched products.
const (
	AlertFloor = "floor" // signal is a possible floor
	AlertDrop  = "drop"  // at or past the watch's drop threshold
)

// alertsFor lists why a watched product needs attention.
func alertsFor(t *Trend, w *db.Watch) []string {
	alerts := []string{}
	if t == nil || w == nil {
		return alerts
	}
	if t.Signal == SignalFloor {
		alerts = append(alerts, AlertFloor)
	}
	if w.DropAlert != nil && t.DropFromRelease >= *w.DropAlert {
		alerts = append(alerts, AlertDrop)
	}
	return alerts
}

type trendsResponse struct {
	Products []trendRow `json:"products"`
	// Notifications says whether phone alerts (ntfy) are set up.
	Notifications bool `json:"notifications"`
	Backfilled    int  `json:"backfilled"`
	Tracked       int  `json:"tracked"`
	Thresholds    struct {
		NewSetDays  int                  `json:"newSetDays"`
		TrendWindow int                  `json:"trendWindow"`
		TrendMove   float64              `json:"trendMove"`
		FloorRules  map[string]FloorRule `json:"floorRules"`
	} `json:"thresholds"`
}

// List returns packs and boxes released in the last `years` years (default 2)
// with their trend summaries, newest set first.
func (h *TrendsHandler) List(w http.ResponseWriter, r *http.Request) {
	years := 2
	if y, err := strconv.Atoi(r.URL.Query().Get("years")); err == nil && y >= 1 && y <= 30 {
		years = y
	}
	c, err := getCatalog()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	today := h.now().UTC().Truncate(24 * time.Hour)
	cutoff := today.AddDate(-years, 0, 0)

	var picked []SearchResult
	var ids []int
	all := trackedProducts(c)
	for _, p := range all {
		rel, err := time.Parse("2006-01-02", p.ReleasedAt)
		if err != nil || rel.Before(cutoff) {
			continue
		}
		picked = append(picked, p)
		ids = append(ids, p.MTGStocksID)
	}

	history, err := db.PriceHistory(r.Context(), h.pool, ids, cutoff.AddDate(0, -3, 0))
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	done, err := db.BackfilledIDs(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	watches, err := db.ListWatches(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	collection, err := db.ListCollection(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	stock := newStockIndex(collection)

	resp := trendsResponse{Products: make([]trendRow, 0, len(picked)), Tracked: len(all), Notifications: notificationsOn()}
	for _, p := range all {
		if done[p.MTGStocksID] {
			resp.Backfilled++
		}
	}
	th := &resp.Thresholds
	th.NewSetDays, th.TrendWindow, th.TrendMove = newSetDays, trendWindow, trendMove
	th.FloorRules = floorRules
	for _, p := range picked {
		var released *time.Time
		if rel, err := time.Parse("2006-01-02", p.ReleasedAt); err == nil {
			released = &rel
		}
		trend := summarize(history[p.MTGStocksID], released, today, floorRules[ruleKind(p.ProductType)])
		row := trendRow{SearchResult: p, Trend: trend, Stock: stock.of(p)}
		if wt, ok := watches[p.MTGStocksID]; ok {
			row.Watch = &wt
		}
		row.Alerts = alertsFor(row.Trend, row.Watch)
		resp.Products = append(resp.Products, row)
	}
	sort.SliceStable(resp.Products, func(i, j int) bool {
		return resp.Products[i].ReleasedAt > resp.Products[j].ReleasedAt
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SetWatch starts, updates (body {"watch":true,"dropAlert":0.3}) or stops
// ({"watch":false}) watching a product.
func (h *TrendsHandler) SetWatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("mtgstocksId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Watch     bool     `json:"watch"`
		DropAlert *float64 `json:"dropAlert"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.DropAlert != nil && (*body.DropAlert <= 0 || *body.DropAlert >= 1) {
		http.Error(w, "drop alert must be between 1% and 99%", http.StatusBadRequest)
		return
	}
	if body.Watch {
		err = db.SetWatch(r.Context(), h.pool, db.Watch{MTGStocksID: id, DropAlert: body.DropAlert})
	} else {
		err = db.DeleteWatch(r.Context(), h.pool, id)
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if body.Watch {
		markAlertsSeen(r.Context(), h.pool, []int{id})
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetWatchMany starts or stops watching many products at once
// (body {"ids":[1,2],"watch":true}); drop alerts of watched ones are kept.
func (h *TrendsHandler) SetWatchMany(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs   []int `json:"ids"`
		Watch bool  `json:"watch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 || len(body.IDs) > 2000 {
		http.Error(w, "ids required", http.StatusBadRequest)
		return
	}
	var err error
	if body.Watch {
		err = db.WatchMany(r.Context(), h.pool, body.IDs)
	} else {
		err = db.UnwatchMany(r.Context(), h.pool, body.IDs)
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if body.Watch {
		markAlertsSeen(r.Context(), h.pool, body.IDs)
	}
	w.WriteHeader(http.StatusNoContent)
}

// History returns one product's full daily price history, for its chart.
func (h *TrendsHandler) History(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("mtgstocksId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	history, err := db.PriceHistory(r.Context(), h.pool, []int{id}, time.Time{})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	points := history[id]
	if points == nil {
		points = []db.PricePoint{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(points)
}

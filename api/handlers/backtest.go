package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Quarterly backtest of the floor rules on the server's own price history.
// For each product kind it replays every candidate rule day by day (using
// only data up to each day) and measures how far above the true post-release
// low it would have bought. Rules are compared by leave-one-release-year-out
// cross-validation; a clearly better rule is proposed, never applied, until
// the admin approves it on the trends page.

const (
	btHorizon      = 540  // days after release the low is searched in
	btMinHistory   = 120  // a product needs this much post-release history...
	btMaxLateStart = 30   // ...starting within this many days of release
	btJudgeAfter   = 30   // a rule is judged on a product only if it has this many days past the rule's minimum age
	btFalseFloor   = 0.10 // price later fell this much more = false floor
	btHit          = 0.05 // an alert within this of the low is a hit
	btMinProducts  = 30   // fewer products than this: too little to tune on
	btMinCoverage  = 0.40 // a rule must fire on at least this share of products
	btMargin       = 0.03 // a proposal must beat the current score by this
	backtestEvery  = 90 * 24 * time.Hour
)

var btGrid = func() []FloorRule {
	var g []FloorRule
	for _, drop := range []float64{0, 0.10, 0.20} {
		for _, window := range []int{14, 28} {
			for _, band := range []float64{0.06, 0.08, 0.12, 0.20} {
				for _, near := range []float64{0.05, 0.10, 0.20} {
					for _, age := range []int{90, 120, 150, 180} {
						for _, recent := range []int{14, 30, 60, 0} {
							g = append(g, FloorRule{MinAge: age, Drop: drop, Window: window, Band: band, Near: near, Recent: recent})
						}
					}
				}
			}
		}
	}
	return g
}()

// ── Active rules ──────────────────────────────────────────────

// defaultFloorRules come from the manual backtest (195 products, sets 2019 to
// mid 2026, leave-one-release-year-out CV, scored on alerts landing within 5%
// of the eventual low). They seed the database and apply until a backtest
// proposal is approved.
//   - regular: within 5% of the low on 31% of alerts (vs 15% before), median
//     ~10% above it; fires on ~half of products.
//   - collector: the earlier rule plus a recent low; within 5% on 26% (vs 16%).
//
// "Low within 60 days" keeps a low that came and went months ago from
// reading as a floor.
var defaultFloorRules = map[string]FloorRule{
	"regular":   {MinAge: 180, Drop: 0.20, Window: 14, Band: 0.20, Near: 0.05, Recent: 60},
	"collector": {MinAge: 150, Drop: 0.00, Window: 14, Band: 0.08, Near: 0.20, Recent: 60},
}

// seededMarker marks an active rule that is a code default rather than one the
// admin approved: newer defaults replace it on startup.
var seededMarker = json.RawMessage(`{"seeded":true}`)

var (
	rulesMu    sync.RWMutex
	floorRules = cloneRules(defaultFloorRules)
)

func cloneRules(m map[string]FloorRule) map[string]FloorRule {
	out := make(map[string]FloorRule, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ruleFor returns the active floor rule for a product type.
func ruleFor(productType string) FloorRule {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	return floorRules[ruleKind(productType)]
}

// activeRules returns a copy of all active rules.
func activeRules() map[string]FloorRule {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	return cloneRules(floorRules)
}

func setRule(kind string, r FloorRule) {
	rulesMu.Lock()
	floorRules[kind] = r
	rulesMu.Unlock()
}

// LoadFloorRules reads the active rules from the database, seeding the
// defaults for any kind that has none yet.
func LoadFloorRules(ctx context.Context, pool *pgxpool.Pool) error {
	stored, err := db.RulesByStatus(ctx, pool, "active")
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, s := range stored {
		var r FloorRule
		if err := json.Unmarshal(s.Rule, &r); err != nil {
			return fmt.Errorf("rule %d: %w", s.ID, err)
		}
		var marks struct {
			Seeded bool `json:"seeded"`
		}
		_ = json.Unmarshal(s.Metrics, &marks)
		if def, ok := defaultFloorRules[s.Kind]; ok && marks.Seeded && r != def {
			continue // an outdated default: replaced below
		}
		setRule(s.Kind, r)
		have[s.Kind] = true
	}
	for kind, r := range defaultFloorRules {
		if !have[kind] {
			raw, _ := json.Marshal(r)
			if err := db.SetActiveRule(ctx, pool, kind, raw, seededMarker); err != nil {
				return err
			}
			setRule(kind, r)
		}
	}
	return nil
}

// ── Data ──────────────────────────────────────────────────────

// btProduct is one product's daily prices from release, ready to replay.
type btProduct struct {
	year   string    // release year, for cross-validation
	offset int       // days from release to the first price
	s      []float64 // daily prices, forward-filled, up to btHorizon
	sufMin []float64 // lowest price from each day on
	rollHi map[int][]float64
	rollLo map[int][]float64
}

func newBTProduct(year string, offset int, s []float64, windows []int) btProduct {
	p := btProduct{year: year, offset: offset, s: s, sufMin: make([]float64, len(s)),
		rollHi: map[int][]float64{}, rollLo: map[int][]float64{}}
	m := math.Inf(1)
	for i := len(s) - 1; i >= 0; i-- {
		m = math.Min(m, s[i])
		p.sufMin[i] = m
	}
	for _, w := range windows {
		hi, lo := make([]float64, len(s)), make([]float64, len(s))
		for i := range s {
			h, l := s[i], s[i]
			for j := max(0, i-w); j < i; j++ {
				h, l = math.Max(h, s[j]), math.Min(l, s[j])
			}
			hi[i], lo[i] = h, l
		}
		p.rollHi[w], p.rollLo[w] = hi, lo
	}
	return p
}

// btSeries turns a product's history into a dense daily series from release,
// or false if it doesn't start near release or doesn't run long enough.
func btSeries(points []db.PricePoint, released time.Time) (offset int, s []float64, ok bool) {
	i := sort.Search(len(points), func(i int) bool { return !points[i].Day.Before(released) })
	if i == len(points) {
		return 0, nil, false
	}
	pts := points[i:]
	first, last := pts[0].Day, pts[len(pts)-1].Day
	offset = int(first.Sub(released).Hours() / 24)
	if offset > btMaxLateStart || last.Sub(released) < btMinHistory*24*time.Hour {
		return 0, nil, false
	}
	end := released.AddDate(0, 0, btHorizon)
	if last.Before(end) {
		end = last
	}
	j := 0
	price := pts[0].Price
	for d := first; !d.After(end); d = d.AddDate(0, 0, 1) {
		for j < len(pts) && !pts[j].Day.After(d) {
			price = pts[j].Price
			j++
		}
		s = append(s, price)
	}
	return offset, s, true
}

// ── Replay and scoring ────────────────────────────────────────

// btOutcome is one product's result under a rule.
type btOutcome struct {
	judged  bool // the product is old enough to judge this rule on
	fired   bool
	premium float64 // price paid over the low (lowest seen within the horizon)
	false_  bool    // price later fell btFalseFloor further
}

// replay walks a product's history day by day, using only data up to each
// day, and reports when the rule would first have fired. Young products are
// only judged on rules they're old enough for: their low may still be ahead.
func replay(p btProduct, r FloorRule) btOutcome {
	if p.offset+len(p.s) < r.MinAge+btJudgeAfter {
		return btOutcome{}
	}
	rel := p.s[0]
	low, lowAt := p.s[0], 0
	hi, lo := p.rollHi[r.Window], p.rollLo[r.Window]
	for i, price := range p.s {
		if price < low {
			low, lowAt = price, i
		}
		if i+p.offset < r.MinAge || i < r.Window {
			continue
		}
		if 1-price/rel >= r.Drop && hi[i]/lo[i]-1 <= r.Band && price/low-1 <= r.Near &&
			(r.Recent <= 0 || i-lowAt <= r.Recent) {
			return btOutcome{judged: true, fired: true, premium: price/p.sufMin[0] - 1, false_: p.sufMin[i] < price*(1-btFalseFloor)}
		}
	}
	return btOutcome{judged: true}
}

// BTMetrics summarizes a rule over some products.
type BTMetrics struct {
	Products  int     `json:"products"`  // products it could be judged on
	Coverage  float64 `json:"coverage"`  // share it fired on
	Hit5      float64 `json:"hit5"`      // share of firings within 5% of the low
	Hit10     float64 `json:"hit10"`     // share of firings within 10% of the low
	Median    float64 `json:"median"`    // median premium over the low
	Mean      float64 `json:"mean"`      // mean premium over the low
	FalseRate float64 `json:"falseRate"` // share of firings the price later fell 10%+ below
}

func metricsOf(outs []btOutcome) BTMetrics {
	var prem []float64
	falses, judged, hit5, hit10 := 0, 0, 0, 0
	for _, o := range outs {
		if !o.judged {
			continue
		}
		judged++
		if o.fired {
			prem = append(prem, o.premium)
			if o.false_ {
				falses++
			}
			if o.premium <= btHit {
				hit5++
			}
			if o.premium <= 2*btHit {
				hit10++
			}
		}
	}
	m := BTMetrics{Products: judged}
	if judged > 0 {
		m.Coverage = float64(len(prem)) / float64(judged)
	}
	if len(prem) > 0 {
		sort.Float64s(prem)
		if n := len(prem); n%2 == 1 {
			m.Median = prem[n/2]
		} else {
			m.Median = (prem[n/2-1] + prem[n/2]) / 2
		}
		sum := 0.0
		for _, v := range prem {
			sum += v
		}
		m.Mean = sum / float64(len(prem))
		m.FalseRate = float64(falses) / float64(len(prem))
		m.Hit5 = float64(hit5) / float64(len(prem))
		m.Hit10 = float64(hit10) / float64(len(prem))
	}
	return m
}

// score: lower is better. The goal is alerts that land within 5% of the
// bottom; the average premium breaks ties between rules that hit as often.
func (m BTMetrics) score() float64 {
	if m.Coverage == 0 {
		return math.Inf(1)
	}
	return -m.Hit5 + 0.30*m.Mean
}

func evaluateRule(products []btProduct, r FloorRule) []btOutcome {
	outs := make([]btOutcome, len(products))
	for i, p := range products {
		outs[i] = replay(p, r)
	}
	return outs
}

// bestRule is the grid rule with the best score on products, among those
// firing on at least btMinCoverage of them; fallback if none qualifies.
func bestRule(products []btProduct, fallback FloorRule) FloorRule {
	best, bestScore := fallback, math.Inf(1)
	for _, r := range btGrid {
		m := metricsOf(evaluateRule(products, r))
		if m.Coverage < btMinCoverage {
			continue
		}
		if s := m.score(); s < bestScore {
			best, bestScore = r, s
		}
	}
	return best
}

// crossValidate tests tuning by holding out one release year at a time:
// each year's products are scored by the rule tuned on the other years.
// Returns the pooled metrics of that procedure and of the fixed current rule.
func crossValidate(products []btProduct, current FloorRule) (tuned, fixed BTMetrics) {
	years := map[string]bool{}
	for _, p := range products {
		years[p.year] = true
	}
	var tunedOuts, fixedOuts []btOutcome
	for y := range years {
		var train, test []btProduct
		for _, p := range products {
			if p.year == y {
				test = append(test, p)
			} else {
				train = append(train, p)
			}
		}
		rule := current
		if len(train) > 0 {
			rule = bestRule(train, current)
		}
		tunedOuts = append(tunedOuts, evaluateRule(test, rule)...)
		fixedOuts = append(fixedOuts, evaluateRule(test, current)...)
	}
	return metricsOf(tunedOuts), metricsOf(fixedOuts)
}

// ── The quarterly run ─────────────────────────────────────────

// BacktestKind is one product kind's backtest result.
type BacktestKind struct {
	Products int        `json:"products"`
	Current  FloorRule  `json:"current"`
	Best     *FloorRule `json:"best,omitempty"`
	// Cross-validated results of keeping the current rule, and of tuning.
	CurrentMetrics BTMetrics `json:"currentMetrics"`
	TunedMetrics   BTMetrics `json:"tunedMetrics"`
	Proposed       bool      `json:"proposed"`
	Note           string    `json:"note,omitempty"`
}

// BacktestSummary is what one run found.
type BacktestSummary struct {
	RanAt time.Time               `json:"ranAt"`
	Kinds map[string]BacktestKind `json:"kinds"`
}

// RunBacktest re-tunes the floor rules on the stored price history and
// proposes any rule that clearly beats the active one. It never changes the
// active rules itself.
func RunBacktest(ctx context.Context, pool *pgxpool.Pool) (*BacktestSummary, error) {
	c, err := getCatalog()
	if err != nil {
		return nil, err
	}
	horizonCutoff := time.Now().UTC().AddDate(0, 0, -btMinHistory) // newer sets can't be judged yet
	byKind := map[string][]SearchResult{}
	var ids []int
	for _, p := range trackedProducts(c) {
		rel, err := time.Parse("2006-01-02", p.ReleasedAt)
		if err != nil || rel.Year() < 2019 || rel.After(horizonCutoff) {
			continue
		}
		kind := ruleKind(p.ProductType)
		byKind[kind] = append(byKind[kind], p)
		ids = append(ids, p.MTGStocksID)
	}
	history, err := db.PriceHistory(ctx, pool, ids, time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, err
	}

	windows := []int{14, 28}
	summary := &BacktestSummary{RanAt: time.Now().UTC(), Kinds: map[string]BacktestKind{}}
	var proposals []string
	rules := activeRules()
	for kind, current := range rules {
		var products []btProduct
		for _, p := range byKind[kind] {
			rel, _ := time.Parse("2006-01-02", p.ReleasedAt)
			if offset, s, ok := btSeries(history[p.MTGStocksID], rel); ok {
				products = append(products, newBTProduct(p.ReleasedAt[:4], offset, s, windows))
			}
		}
		res := BacktestKind{Products: len(products), Current: current}
		if len(products) < btMinProducts {
			res.Note = fmt.Sprintf("only %d products with enough history; need %d", len(products), btMinProducts)
			summary.Kinds[kind] = res
			continue
		}
		res.TunedMetrics, res.CurrentMetrics = crossValidate(products, current)
		best := bestRule(products, current)
		res.Best = &best
		if best != current && res.TunedMetrics.score()+btMargin < res.CurrentMetrics.score() {
			rule, _ := json.Marshal(best)
			metrics, _ := json.Marshal(map[string]any{
				"current": res.CurrentMetrics, "proposed": res.TunedMetrics, "products": len(products),
			})
			if _, err := db.ProposeRule(ctx, pool, kind, rule, metrics); err != nil {
				return nil, err
			}
			res.Proposed = true
			proposals = append(proposals, fmt.Sprintf("%s: %.0f%% → %.0f%% above the low (median)",
				kind, res.CurrentMetrics.Median*100, res.TunedMetrics.Median*100))
		}
		summary.Kinds[kind] = res
	}

	raw, _ := json.Marshal(summary)
	if err := db.RecordBacktest(ctx, pool, raw); err != nil {
		return nil, err
	}
	if len(proposals) > 0 && notificationsOn() {
		msg := strings.Join(proposals, "\n") + "\nReview it on Price Trends."
		if err := sendNotification(ctx, "New floor rule proposed", msg); err != nil {
			log.Printf("backtest: notifying: %v", err)
		}
	}
	log.Printf("backtest: done (%d proposals)", len(proposals))
	return summary, nil
}

// MaybeRunBacktest runs the backtest when the last run is a quarter old (or
// it never ran), once price history is fully backfilled.
func MaybeRunBacktest(ctx context.Context, pool *pgxpool.Pool) error {
	last, _, err := db.LastBacktest(ctx, pool)
	if err != nil || time.Since(last) < backtestEvery {
		return err
	}
	c, err := getCatalog()
	if err != nil {
		return err
	}
	done, err := db.BackfilledIDs(ctx, pool)
	if err != nil {
		return err
	}
	for _, p := range trackedProducts(c) {
		if !done[p.MTGStocksID] {
			return nil // wait for the backfill: tuning on partial history would mislead
		}
	}
	_, err = RunBacktest(ctx, pool)
	return err
}

// ── API ───────────────────────────────────────────────────────

// RuleProposal is an open proposal shown on the trends page.
type RuleProposal struct {
	ID        int             `json:"id"`
	Kind      string          `json:"kind"`
	Rule      FloorRule       `json:"rule"`
	Current   FloorRule       `json:"current"`
	Metrics   json.RawMessage `json:"metrics"`
	CreatedAt time.Time       `json:"createdAt"`
}

// openProposals lists proposals with the rule each would replace.
func openProposals(ctx context.Context, pool *pgxpool.Pool) ([]RuleProposal, error) {
	stored, err := db.RulesByStatus(ctx, pool, "proposed")
	if err != nil {
		return nil, err
	}
	rules := activeRules()
	out := make([]RuleProposal, 0, len(stored))
	for _, s := range stored {
		var r FloorRule
		if err := json.Unmarshal(s.Rule, &r); err != nil {
			continue
		}
		out = append(out, RuleProposal{ID: s.ID, Kind: s.Kind, Rule: r, Current: rules[s.Kind], Metrics: s.Metrics, CreatedAt: s.CreatedAt})
	}
	return out, nil
}

// Backtest runs the backtest now and returns what it found.
func (h *TrendsHandler) Backtest(w http.ResponseWriter, r *http.Request) {
	summary, err := RunBacktest(r.Context(), h.pool)
	if err != nil {
		log.Printf("backtest: %v", err)
		http.Error(w, "backtest failed: try again once prices load", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

// ApplyRule makes a proposal the active rule for its kind, right away.
func (h *TrendsHandler) ApplyRule(w http.ResponseWriter, r *http.Request) { h.decide(w, r, true) }

// DismissRule closes a proposal without changing anything.
func (h *TrendsHandler) DismissRule(w http.ResponseWriter, r *http.Request) { h.decide(w, r, false) }

func (h *TrendsHandler) decide(w http.ResponseWriter, r *http.Request, apply bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	decided, err := db.DecideProposal(r.Context(), h.pool, id, apply)
	if errors.Is(err, db.ErrNoRule) {
		http.Error(w, "that proposal was already decided or replaced", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if apply {
		var rule FloorRule
		if err := json.Unmarshal(decided.Rule, &rule); err == nil {
			setRule(decided.Kind, rule)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mtg-chaos-draft/db"
)

// Best deals: every buyable single pack (in-stock store listings, plus
// TCGplayer at its market price), scored on what you get for your money now
// and whether waiting would likely make it cheaper. The page adds shipping,
// applies the viewer's filters and picks the top 3 from different sets.
//
//	score = contents ÷ (price + shipping per pack) × timing
//
// timing is 1 + the median price change over the following 90 days for packs
// with the same signal, measured on real histories (2019–2026, every 14 days,
// TestRealHistories data): buying a climber now beats buying it later; a new
// set is usually cheaper in 3 months. Small-sample cases (dips) aren't used.
var timingFactors = map[string]map[Signal]float64{
	"collector": {SignalClimbing: 1.12, SignalRising: 1.077, SignalFloor: 1.035, SignalStable: 1.032, SignalFalling: 1.028, SignalNew: 0.952},
	"regular":   {SignalClimbing: 1.082, SignalRising: 1.035, SignalFloor: 1.032, SignalStable: 1.021, SignalFalling: 1.045, SignalNew: 0.935},
}

// timingFor is a pack's timing factor (1 without a signal).
func timingFor(productType string, s Signal) float64 {
	if f, ok := timingFactors[ruleKind(productType)][s]; ok {
		return f
	}
	return 1
}

// shippingDefaults: rough per-order shipping to the US, editable on the page.
// CardTrader Zero's smallest US envelope is $3.37; sealed product likely
// needs more. Card Kingdom's free shipping ($75) covers singles, not packs.
var shippingDefaults = map[string]float64{"Card Kingdom": 7, cardTraderName: 5, "TCGplayer": 4}

// Candidate is one way to buy one pack.
type Candidate struct {
	MTGStocksID int      `json:"mtgstocksId"`
	SetName     string   `json:"setName"`
	ProductType string   `json:"productType"`
	ReleasedAt  string   `json:"releasedAt"`
	Store       string   `json:"store"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Language    string   `json:"language"`
	Price       float64  `json:"price"`
	Market      float64  `json:"market"`
	Contents    float64  `json:"contents"`
	Signal      Signal   `json:"signal"`
	Timing      float64  `json:"timing"`
	EVChange    *float64 `json:"evChange,omitempty"`
}

// BestDeals lists scoring candidates (GET /api/best-deals).
func (h *TrendsHandler) BestDeals(w http.ResponseWriter, r *http.Request) {
	c, err := getCatalog()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	today := h.now().UTC().Truncate(24 * time.Hour)
	// Released, sales-backed single packs with a contents value.
	var packs []SearchResult
	for _, p := range trackedProducts(c) {
		if p.PacksPerBox > 0 || p.MarketPrice == nil || *p.MarketPrice <= 0 || p.PriceDoubt != "" || p.ReleasedAt > today.Format(time.DateOnly) {
			continue
		}
		if packValue(p.MTGStocksID) > 0 {
			packs = append(packs, p)
		}
	}
	rows, err := h.rows(r.Context(), c, packs, nil, time.Time{}, today)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	offers, err := db.ListStoreOffers(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	listings := map[int][]Deal{}
	for _, d := range offersWithMarket(offers, c.products) {
		if d.Available {
			listings[d.MTGStocksID] = append(listings[d.MTGStocksID], d)
		}
	}

	out := []Candidate{}
	for _, row := range rows {
		p := row.SearchResult
		base := Candidate{MTGStocksID: p.MTGStocksID, SetName: p.SetName, ProductType: p.ProductType, ReleasedAt: p.ReleasedAt,
			Market: *p.MarketPrice, Contents: packValue(p.MTGStocksID), Timing: 1}
		if row.Trend != nil {
			base.Signal = row.Trend.Signal
			base.Timing = timingFor(p.ProductType, row.Trend.Signal)
		}
		if row.EV != nil {
			base.EVChange = row.EV.Change
		}
		// TCGplayer at its market price: what most sellers sell at.
		tcg := base
		tcg.Store, tcg.Language, tcg.Price, tcg.Title = "TCGplayer", "en", base.Market, p.SetName+" "+p.ProductType+" (market price)"
		tcg.URL = tcgLink(p.MTGStocksID)
		out = append(out, tcg)
		for _, d := range listings[p.MTGStocksID] {
			cand := base
			cand.Store, cand.URL, cand.Title, cand.Language, cand.Price = d.Store, d.URL, d.Title, d.Language, d.Price
			out = append(out, cand)
		}
	}
	// Best first by contents per dollar before shipping (the page re-ranks).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Contents/out[i].Price*out[i].Timing > out[j].Contents/out[j].Price*out[j].Timing
	})
	resp := struct {
		Candidates []Candidate                   `json:"candidates"`
		Shipping   map[string]float64            `json:"shipping"`
		AsOf       string                        `json:"asOf"`
		Timing     map[string]map[Signal]float64 `json:"timing"`
	}{out, shippingDefaults, today.Format(time.DateOnly), timingFactors}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Card Kingdom's whole sealed list is one request: re-read at most every
// ctLiveCache for live checks.
var (
	ckLiveMu    sync.Mutex
	ckLiveAt    time.Time
	ckLiveOffer []db.Offer
)

// liveCardKingdom returns Card Kingdom's current listings for the given packs.
func liveCardKingdom(ctx context.Context, idx *packIndex, ids []int) ([]db.Offer, error) {
	if cardKingdomURL == "" {
		return []db.Offer{}, nil
	}
	ckLiveMu.Lock()
	defer ckLiveMu.Unlock()
	if ckLiveOffer == nil || ctLiveClock().Sub(ckLiveAt) >= ctLiveCache {
		offers, _, err := fetchCardKingdom(ctx, idx)
		if err != nil {
			return nil, err
		}
		now := ctLiveClock().UTC()
		for i := range offers {
			offers[i].SeenAt = now
		}
		ckLiveOffer, ckLiveAt = offers, now
	}
	want := map[int]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := []db.Offer{}
	for _, o := range ckLiveOffer {
		if want[o.MTGStocksID] {
			out = append(out, o)
		}
	}
	return out, nil
}

// BestDealsLive re-checks CardTrader and Card Kingdom live for the packs a page is showing
// (GET /api/best-deals/live?ids=1,2,3; at most ctLiveMax) and returns their
// current listings, with the usual rules applied. Nothing is saved.
func (h *TrendsHandler) BestDealsLive(w http.ResponseWriter, r *http.Request) {
	var ids []int
	for _, f := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		http.Error(w, "ids required", http.StatusBadRequest)
		return
	}
	c, err := getCatalog()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	idx := newPackIndex(c.products)
	fresh, err := liveCardTrader(r.Context(), h.pool, idx, ids)
	if err != nil {
		log.Printf("best deals: live CardTrader check: %v", err)
		http.Error(w, "couldn't reach CardTrader right now", http.StatusBadGateway)
		return
	}
	ck, err := liveCardKingdom(r.Context(), idx, ids)
	if err != nil {
		log.Printf("best deals: live Card Kingdom check: %v", err)
		http.Error(w, "couldn't reach Card Kingdom right now", http.StatusBadGateway)
		return
	}
	fresh = append(fresh, ck...)
	deals := []Deal{}
	for _, d := range offersWithMarket(fresh, c.products) {
		if d.Available {
			deals = append(deals, d)
		}
	}
	resp := struct {
		Offers    []Deal    `json:"offers"`
		CheckedAt time.Time `json:"checkedAt"`
	}{deals, time.Now().UTC()}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

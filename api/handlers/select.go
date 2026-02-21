package handlers

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SelectHandler struct {
	pool *pgxpool.Pool
}

func NewSelectHandler(pool *pgxpool.Pool) *SelectHandler {
	return &SelectHandler{pool: pool}
}

func (h *SelectHandler) Select(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PackIDs []int `json:"packIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.PackIDs) == 0 {
		http.Error(w, "bad request: packIds required", http.StatusBadRequest)
		return
	}

	packs, err := db.GetPacksByIDs(r.Context(), h.pool, body.PackIDs)
	if err != nil || len(packs) == 0 {
		http.Error(w, "no packs found", http.StatusBadRequest)
		return
	}

	settings, err := db.GetWeightSettings(r.Context(), h.pool)
	if err != nil {
		// Non-fatal: fall back to base weights
		settings = &db.WeightSettings{}
	}

	weights := effectiveWeights(packs, settings.PriceSensitivity, settings.ScaricitySensitivity)
	selected := weightedRandom(packs, weights)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"selectedId":   selected.ID,
		"selectedPack": selected,
	})
}

// effectiveWeights computes a weight for each pack based on base weight, price, and quantity.
//
// Formula (per pack i):
//   price_factor   = (1 - pS) + pS × (minPrice / price_i)   [1.0 if no price or pS=0]
//   qty_factor     = (1 - sS) + sS × (qty_i / maxQty)        [1.0 if sS=0]
//   effective      = base_weight × price_factor × qty_factor
//
// The cheapest pack and most-stocked pack always get factor 1.0; others are penalized relative to them.
func effectiveWeights(packs []db.CollectionPack, priceSensitivity, scarcitySensitivity float64) []float64 {
	// Gather prices and quantities
	minPrice := math.MaxFloat64
	maxQty := 0
	for _, p := range packs {
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			if *p.MarketPrice < minPrice {
				minPrice = *p.MarketPrice
			}
		}
		if p.Quantity > maxQty {
			maxQty = p.Quantity
		}
	}
	if minPrice == math.MaxFloat64 {
		minPrice = 0
	}

	weights := make([]float64, len(packs))
	for i, p := range packs {
		priceFactor := 1.0
		if priceSensitivity > 0 && p.MarketPrice != nil && *p.MarketPrice > 0 && minPrice > 0 {
			priceFactor = (1-priceSensitivity) + priceSensitivity*(minPrice / *p.MarketPrice)
		}

		qtyFactor := 1.0
		if scarcitySensitivity > 0 && maxQty > 0 {
			qtyFactor = (1-scarcitySensitivity) + scarcitySensitivity*(float64(p.Quantity)/float64(maxQty))
		}

		weights[i] = p.Weight * priceFactor * qtyFactor
	}
	return weights
}

func weightedRandom(packs []db.CollectionPack, weights []float64) db.CollectionPack {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := rand.Float64() * total
	for i, w := range weights {
		r -= w
		if r <= 0 {
			return packs[i]
		}
	}
	return packs[len(packs)-1]
}

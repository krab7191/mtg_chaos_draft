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

	weights := effectiveWeights(packs)
	selected := weightedRandom(packs, weights)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"selectedId":   selected.ID,
		"selectedPack": selected,
	})
}

// effectiveWeights computes odds as min(priceOdds, scarcityOdds) per pack, then returns
// those as raw weights for weightedRandom (which renormalises internally).
//
// priceOdds:    normalised 1/price  — expensive packs are less likely
// scarcityOdds: normalised 1/qty    — scarce packs are less likely
// The smaller of the two applies, so a pack is penalised if it is EITHER expensive OR scarce.
// Packs with qty=0 get weight 0 (excluded). Unpriced packs use average price as fallback.
func effectiveWeights(packs []db.CollectionPack) []float64 {
	// Avg price fallback for unpriced packs
	var priceSum float64
	var priceCount int
	for _, p := range packs {
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			priceSum += *p.MarketPrice
			priceCount++
		}
	}
	avgPrice := 1.0
	if priceCount > 0 {
		avgPrice = priceSum / float64(priceCount)
	}

	pw := make([]float64, len(packs)) // price weights
	sw := make([]float64, len(packs)) // scarcity weights
	var priceTotal, scarcityTotal float64
	for i, p := range packs {
		price := avgPrice
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			price = *p.MarketPrice
		}
		pw[i] = 1.0 / price
		priceTotal += pw[i]

		if p.Quantity > 0 {
			sw[i] = 1.0 / float64(p.Quantity)
			scarcityTotal += sw[i]
		}
	}

	result := make([]float64, len(packs))
	for i := range packs {
		if sw[i] == 0 {
			result[i] = 0 // qty=0: excluded
			continue
		}
		priceOdds := pw[i] / priceTotal
		scarcityOdds := sw[i] / scarcityTotal
		result[i] = math.Min(priceOdds, scarcityOdds)
	}
	return result
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

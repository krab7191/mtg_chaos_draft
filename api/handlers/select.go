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
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	weights := effectiveWeights(packs, settings)
	selected := weightedRandom(packs, weights)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"selectedId":   selected.ID,
		"selectedPack": selected,
	})
}

// effectiveWeights computes odds as min(priceOdds, scarcityOdds) per pack.
// Caps clamp price and quantity to a maximum before weighting — packs above the
// cap are treated as if they cost/have exactly the cap value, so they aren't
// penalised further. A cap of 0 means no cap.
func effectiveWeights(packs []db.CollectionPack, settings *db.WeightSettings) []float64 {
	capPrice := func(price float64) float64 {
		if settings.PriceFloor > 0 && price < settings.PriceFloor {
			price = settings.PriceFloor
		}
		if settings.PriceCap > 0 && price > settings.PriceCap {
			price = settings.PriceCap
		}
		return price
	}
	capQty := func(qty int) int {
		if settings.QuantityCap > 0 && qty > settings.QuantityCap {
			return settings.QuantityCap
		}
		return qty
	}

	// Avg price fallback for unpriced packs
	var priceSum float64
	var priceCount int
	for _, p := range packs {
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			priceSum += capPrice(*p.MarketPrice)
			priceCount++
		}
	}
	avgPrice := 1.0
	if priceCount > 0 {
		avgPrice = priceSum / float64(priceCount)
	}

	pw := make([]float64, len(packs))
	sw := make([]float64, len(packs))
	var priceTotal, scarcityTotal float64
	for i, p := range packs {
		price := avgPrice
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			price = capPrice(*p.MarketPrice)
		}
		pw[i] = 1.0 / price
		priceTotal += pw[i]

		qty := capQty(p.Quantity)
		if qty > 0 {
			sw[i] = 1.0 / float64(qty)
			scarcityTotal += sw[i]
		}
	}

	result := make([]float64, len(packs))
	for i, p := range packs {
		if sw[i] == 0 {
			result[i] = 0
			continue
		}
		priceOdds := pw[i] / priceTotal
		scarcityOdds := sw[i] / scarcityTotal
		w := math.Min(priceOdds, scarcityOdds)
		if mult, ok := settings.PackWeights[p.ID]; ok && mult > 0 {
			w *= mult
		}
		result[i] = w
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

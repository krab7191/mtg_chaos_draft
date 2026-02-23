package handlers

import (
	"encoding/json"
	"net/http"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsHandler struct {
	pool *pgxpool.Pool
}

func NewSettingsHandler(pool *pgxpool.Pool) *SettingsHandler {
	return &SettingsHandler{pool: pool}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := db.GetWeightSettings(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PriceSensitivity     float64         `json:"priceSensitivity"`
		ScaricitySensitivity float64         `json:"scarcitySensitivity"`
		PriceCap             float64         `json:"priceCap"`
		PriceFloor           float64         `json:"priceFloor"`
		QuantityCap          int             `json:"quantityCap"`
		PackWeights          map[int]float64 `json:"packWeights"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	clampPos := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		return v
	}

	s := &db.WeightSettings{
		PriceSensitivity:     clamp01(body.PriceSensitivity),
		ScaricitySensitivity: clamp01(body.ScaricitySensitivity),
		PriceCap:             clampPos(body.PriceCap),
		PriceFloor:           clampPos(body.PriceFloor),
		QuantityCap:          max(0, body.QuantityCap),
		PackWeights:          body.PackWeights,
	}
	if s.PackWeights == nil {
		s.PackWeights = map[int]float64{}
	}

	result, err := db.UpdateWeightSettings(r.Context(), h.pool, s)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

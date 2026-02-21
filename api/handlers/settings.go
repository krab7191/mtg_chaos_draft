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
	json.NewEncoder(w).Encode(s)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PriceSensitivity    float64 `json:"priceSensitivity"`
		ScaricitySensitivity float64 `json:"scarcitySensitivity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Clamp to [0, 1]
	clamp := func(v float64) float64 {
		if v < 0 { return 0 }
		if v > 1 { return 1 }
		return v
	}
	s, err := db.UpdateWeightSettings(r.Context(), h.pool,
		clamp(body.PriceSensitivity), clamp(body.ScaricitySensitivity))
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

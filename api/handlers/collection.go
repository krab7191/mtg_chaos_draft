package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CollectionHandler struct {
	pool *pgxpool.Pool
}

func NewCollectionHandler(pool *pgxpool.Pool) *CollectionHandler {
	return &CollectionHandler{pool: pool}
}

func (h *CollectionHandler) List(w http.ResponseWriter, r *http.Request) {
	packs, err := db.ListCollection(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if packs == nil {
		packs = []db.CollectionPack{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(packs)
}

func (h *CollectionHandler) Add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScryfallSetCode string  `json:"scryfallSetCode"`
		Name            string  `json:"name"`
		SetName         string  `json:"setName"`
		ProductType     string  `json:"productType"`
		Quantity        int     `json:"quantity"`
		Weight          float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ScryfallSetCode == "" || body.ProductType == "" {
		http.Error(w, "scryfallSetCode and productType required", http.StatusBadRequest)
		return
	}
	if body.Quantity == 0 {
		body.Quantity = 1
	}
	if body.Weight == 0 {
		body.Weight = 1.0
	}
	pack, err := db.AddPack(r.Context(), h.pool, body.ScryfallSetCode, body.Name, body.SetName, body.ProductType, body.Quantity, body.Weight)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "pack already in collection", http.StatusConflict)
			return
		}
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pack)
}

func (h *CollectionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Quantity    *int     `json:"quantity"`
		Weight      *float64 `json:"weight"`
		Notes       *string  `json:"notes"`
		MTGStocksID *int     `json:"mtgstocksId"`
		MarketPrice *float64 `json:"marketPrice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pack, err := db.UpdatePack(r.Context(), h.pool, id, body.Quantity, body.Weight, body.Notes, body.MTGStocksID, body.MarketPrice)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pack)
}

// LinkPrice saves an MTGStocks ID for a pack and immediately fetches its price.
func (h *CollectionHandler) LinkPrice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		MTGStocksID int `json:"mtgstocksId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MTGStocksID == 0 {
		http.Error(w, "mtgstocksId required", http.StatusBadRequest)
		return
	}

	var pricePtr *float64
	if price, ok := fetchSealedPrice(body.MTGStocksID); ok {
		pricePtr = &price
	}

	pack, err := db.UpdatePack(r.Context(), h.pool, id, nil, nil, nil, &body.MTGStocksID, pricePtr)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pack)
}

func (h *CollectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeletePack(r.Context(), h.pool, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isUniqueViolation(err error) bool {
	return err != nil && len(err.Error()) > 0 &&
		(contains(err.Error(), "23505") || contains(err.Error(), "unique"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

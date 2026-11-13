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
	_ = json.NewEncoder(w).Encode(packs)
}

func (h *CollectionHandler) Add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MTGStocksID int      `json:"mtgstocksId"`
		Name        string   `json:"name"`
		SetName     string   `json:"setName"`
		SetCode     string   `json:"setCode"`
		ProductType string   `json:"productType"`
		Quantity    int      `json:"quantity"`
		Weight      float64  `json:"weight"`
		MarketPrice *float64 `json:"marketPrice"`
		// Optional: what was paid, per pack or in total, and when (YYYY-MM-DD, default today).
		PaidPrice   *float64 `json:"paidPrice"`
		PaidTotal   *float64 `json:"paidTotal"`
		Fees        *float64 `json:"fees"` // tax & shipping within the price paid
		PurchasedOn string   `json:"purchasedOn"`
		// Default true. False logs the purchase without adding stock (packs since opened).
		AddToStock *bool `json:"addToStock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.MTGStocksID == 0 || body.ProductType == "" {
		http.Error(w, "mtgstocksId and productType required", http.StatusBadRequest)
		return
	}
	if body.Quantity == 0 {
		body.Quantity = 1
	}
	total, msg := paidTotal(body.PaidPrice, body.PaidTotal, body.Quantity)
	if msg == "" {
		msg = validateDate(body.PurchasedOn)
	}
	var fees float64
	if msg == "" && total != nil {
		fees, msg = checkFees(body.Fees, *total)
	}
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if body.Weight == 0 {
		body.Weight = 1.0
	}
	stock := body.Quantity
	if body.AddToStock != nil && !*body.AddToStock {
		stock = 0
	}
	cardsPerPack := lookupCardsPerPack(body.SetCode, body.ProductType)
	var pack *db.CollectionPack
	var purchase *db.Purchase
	var err error
	if total != nil {
		pack, purchase, err = db.AddPackWithPurchase(r.Context(), h.pool, body.MTGStocksID, body.Name, body.SetName, body.SetCode, body.ProductType, body.Quantity, stock, body.Weight, body.MarketPrice, cardsPerPack, *total, fees, body.PurchasedOn)
	} else {
		pack, err = db.AddPack(r.Context(), h.pool, body.MTGStocksID, body.Name, body.SetName, body.SetCode, body.ProductType, stock, body.Weight, body.MarketPrice, cardsPerPack)
	}
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// The pack, plus the purchase when one was logged.
	_ = json.NewEncoder(w).Encode(struct {
		*db.CollectionPack
		Purchase *db.Purchase `json:"purchase,omitempty"`
	}{pack, purchase})
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
		MarketPrice *float64 `json:"marketPrice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pack, err := db.UpdatePack(r.Context(), h.pool, id, body.Quantity, body.Weight, body.Notes, nil, body.MarketPrice, nil)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pack)
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

	pack, err := db.UpdatePack(r.Context(), h.pool, id, nil, nil, nil, &body.MTGStocksID, pricePtr, nil)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pack)
}

// RefreshPrice fetches the current MTGStocks price for a single pack, stores it,
// and returns the updated pack. If the live fetch fails, the stored pack is
// returned unchanged so callers can fall back to the last known price.
func (h *CollectionHandler) RefreshPrice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	packs, err := db.GetPacksByIDs(r.Context(), h.pool, []int{id})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if len(packs) == 0 {
		http.Error(w, "pack not found", http.StatusNotFound)
		return
	}
	pack := &packs[0]
	if pack.MTGStocksID != nil {
		if price, ok := fetchSealedPrice(*pack.MTGStocksID); ok {
			updated, err := db.UpdatePack(r.Context(), h.pool, id, nil, nil, nil, nil, &price, nil)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			pack = updated
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pack)
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

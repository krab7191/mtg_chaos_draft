package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SaleHandler lists packs that left the collection (drafts and sales) and
// records sales.
type SaleHandler struct {
	pool *pgxpool.Pool
}

func NewSaleHandler(pool *pgxpool.Pool) *SaleHandler {
	return &SaleHandler{pool: pool}
}

type saleBody struct {
	PackID    int      `json:"packId"`
	Quantity  int      `json:"quantity"`
	SalePrice *float64 `json:"salePrice"` // per pack
	SaleTotal *float64 `json:"saleTotal"` // for all packs
	Fees      *float64 `json:"fees"`      // selling fees & shipping, paid out of the sale
	SoldOn    string   `json:"soldOn"`
}

// check validates a sale and returns its total and fees, or a message for the client.
func (b *saleBody) check() (total, fees float64, msg string) {
	if b.Quantity <= 0 {
		return 0, 0, "quantity must be at least 1"
	}
	t, msg := resolveTotal(b.SalePrice, b.SaleTotal, b.Quantity, "salePrice", "saleTotal")
	if msg != "" {
		return 0, 0, msg
	}
	if t == nil {
		return 0, 0, "salePrice or saleTotal required"
	}
	if msg := validateDate(b.SoldOn); msg != "" {
		return 0, 0, msg
	}
	fees, msg = checkFees(b.Fees, *t)
	return *t, fees, msg
}

func writeSaleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrPackNotFound):
		http.Error(w, "pack not found", http.StatusNotFound)
	case errors.Is(err, db.ErrSaleNotFound):
		http.Error(w, "sale not found", http.StatusNotFound)
	case errors.Is(err, db.ErrNotEnoughStock):
		http.Error(w, "not enough packs in stock", http.StatusConflict)
	default:
		http.Error(w, "db error", http.StatusInternalServerError)
	}
}

// List returns every draft and sale, newest first.
func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := db.ListDisposals(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []db.Disposal{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// Create records a sale and takes the packs out of stock.
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body saleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	total, fees, msg := body.check()
	if msg == "" && body.PackID == 0 {
		msg = "packId required"
	}
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	d, err := db.RecordSale(r.Context(), h.pool, body.PackID, body.Quantity, total, fees, body.SoldOn)
	if err != nil {
		writeSaleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(d)
}

// Update corrects a sale; a change in quantity moves packs in or out of stock.
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body saleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	total, fees, msg := body.check()
	if msg == "" && body.SoldOn == "" {
		msg = "soldOn required"
	}
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	d, err := db.UpdateSale(r.Context(), h.pool, id, body.Quantity, total, fees, body.SoldOn)
	if err != nil {
		writeSaleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

// SetDraftCollected records what was actually collected from players for an
// approved draft (body: {"amount": 60}).
func (h *SaleHandler) SetDraftCollected(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Amount *float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Amount == nil {
		http.Error(w, "amount required", http.StatusBadRequest)
		return
	}
	if *body.Amount < 0 {
		http.Error(w, "amount must not be negative", http.StatusBadRequest)
		return
	}
	list, err := db.SetDraftCollected(r.Context(), h.pool, id, *body.Amount)
	if errors.Is(err, db.ErrDraftNotRecorded) {
		http.Error(w, "draft not approved, or its packs weren't recorded", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// Delete undoes a sale: the packs go back into stock.
func (h *SaleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeleteSale(r.Context(), h.pool, id); err != nil {
		writeSaleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

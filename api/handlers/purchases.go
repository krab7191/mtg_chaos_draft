package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PurchaseHandler records what was paid for packs.
type PurchaseHandler struct {
	pool *pgxpool.Pool
}

func NewPurchaseHandler(pool *pgxpool.Pool) *PurchaseHandler {
	return &PurchaseHandler{pool: pool}
}

// paidTotal works out the total paid from either the per-pack price or the
// total (a display bought for one price). It returns nil when neither is set,
// and a message for the client when the input is invalid.
func paidTotal(perPack, total *float64, quantity int) (*float64, string) {
	return resolveTotal(perPack, total, quantity, "paidPrice", "paidTotal")
}

// resolveTotal turns a per-pack price or a total (named perName / totalName
// in the request) into the total for quantity packs.
func resolveTotal(perPack, total *float64, quantity int, perName, totalName string) (*float64, string) {
	switch {
	case perPack != nil && total != nil:
		return nil, "send " + perName + " or " + totalName + ", not both"
	case total != nil:
		if *total < 0 {
			return nil, totalName + " must not be negative"
		}
		return total, ""
	case perPack != nil:
		if *perPack < 0 {
			return nil, perName + " must not be negative"
		}
		t := *perPack * float64(quantity)
		return &t, ""
	}
	return nil, ""
}

// validateDate checks an optional purchase date, returning a message for the
// client, or "" when it's fine.
func validateDate(purchasedOn string) string {
	if purchasedOn == "" {
		return ""
	}
	d, err := time.Parse(time.DateOnly, purchasedOn)
	if err != nil {
		return "purchasedOn must be YYYY-MM-DD"
	}
	// Allow a day of slack for clients ahead of the server's timezone.
	if d.After(time.Now().AddDate(0, 0, 1)) {
		return "purchasedOn is in the future"
	}
	return ""
}

func (h *PurchaseHandler) List(w http.ResponseWriter, r *http.Request) {
	purchases, err := db.ListPurchases(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if purchases == nil {
		purchases = []db.Purchase{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(purchases)
}

// Create logs a purchase of a pack already in the collection.
func (h *PurchaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PackID      int      `json:"packId"`
		Quantity    int      `json:"quantity"`
		PaidPrice   *float64 `json:"paidPrice"` // per pack
		PaidTotal   *float64 `json:"paidTotal"` // for all packs
		Fees        *float64 `json:"fees"`      // tax & shipping within the total
		PurchasedOn string   `json:"purchasedOn"`
		AddToStock  bool     `json:"addToStock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	total, msg := checkPurchase(body.Quantity, body.PaidPrice, body.PaidTotal, body.PurchasedOn)
	if msg == "" && body.PackID == 0 {
		msg = "packId required"
	}
	var fees float64
	if msg == "" {
		fees, msg = checkFees(body.Fees, total)
	}
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	p, err := db.LogPurchase(r.Context(), h.pool, body.PackID, body.Quantity, total, fees, body.PurchasedOn, body.AddToStock)
	if errors.Is(err, db.ErrPackNotFound) {
		http.Error(w, "pack not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// checkFees validates optional tax & shipping against the total paid.
func checkFees(fees *float64, total float64) (float64, string) {
	if fees == nil {
		return 0, ""
	}
	if *fees < 0 {
		return 0, "fees must not be negative"
	}
	if *fees > total {
		return 0, "fees can't be more than the total paid"
	}
	return *fees, ""
}

// checkPurchase validates the fields every logged purchase needs and returns
// the total paid, or a message for the client.
func checkPurchase(quantity int, perPack, total *float64, purchasedOn string) (float64, string) {
	if quantity <= 0 {
		return 0, "quantity must be at least 1"
	}
	t, msg := paidTotal(perPack, total, quantity)
	if msg != "" {
		return 0, msg
	}
	if t == nil {
		return 0, "paidPrice or paidTotal required"
	}
	if msg := validateDate(purchasedOn); msg != "" {
		return 0, msg
	}
	return *t, ""
}

// Update corrects a logged purchase (quantity, price paid, date).
func (h *PurchaseHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Quantity    int      `json:"quantity"`
		PaidPrice   *float64 `json:"paidPrice"`
		PaidTotal   *float64 `json:"paidTotal"`
		Fees        *float64 `json:"fees"`
		PurchasedOn string   `json:"purchasedOn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	total, msg := checkPurchase(body.Quantity, body.PaidPrice, body.PaidTotal, body.PurchasedOn)
	if msg == "" && body.PurchasedOn == "" {
		msg = "purchasedOn required"
	}
	var fees float64
	if msg == "" {
		fees, msg = checkFees(body.Fees, total)
	}
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	p, err := db.UpdatePurchase(r.Context(), h.pool, id, body.Quantity, total, fees, body.PurchasedOn)
	if errors.Is(err, db.ErrPurchaseNotFound) {
		http.Error(w, "purchase not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (h *PurchaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeletePurchase(r.Context(), h.pool, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

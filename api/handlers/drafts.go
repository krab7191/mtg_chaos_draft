package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"mtg-chaos-draft/db"
	mw "mtg-chaos-draft/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftHandler struct {
	pool *pgxpool.Pool
}

func NewDraftHandler(pool *pgxpool.Pool) *DraftHandler {
	return &DraftHandler{pool: pool}
}

func (h *DraftHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Picks []struct {
			PackID      *int     `json:"packId"`
			SetName     string   `json:"setName"`
			ProductType string   `json:"productType"`
			MarketPrice *float64 `json:"marketPrice"`
		} `json:"picks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.Picks) != 12 {
		http.Error(w, "picks must contain exactly 12 entries", http.StatusBadRequest)
		return
	}

	picks := make([]db.DraftPick, len(body.Picks))
	for i, p := range body.Picks {
		picks[i] = db.DraftPick{
			PackID:      p.PackID,
			SetName:     p.SetName,
			ProductType: p.ProductType,
			MarketPrice: p.MarketPrice,
		}
	}

	draft, err := db.CreateDraft(r.Context(), h.pool, picks)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(draft)
}

func (h *DraftHandler) List(w http.ResponseWriter, r *http.Request) {
	drafts, err := db.ListDrafts(r.Context(), h.pool)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(drafts)
}

func (h *DraftHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeleteDraft(r.Context(), h.pool, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DraftHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	user := mw.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := db.ApproveDraft(r.Context(), h.pool, id, user.ID); err != nil {
		if errors.Is(err, db.ErrDraftNotFound) {
			http.Error(w, "draft not found or already approved", http.StatusNotFound)
			return
		}
		var insuffErr *db.InsufficientQuantityError
		if errors.As(err, &insuffErr) {
			http.Error(w, insuffErr.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

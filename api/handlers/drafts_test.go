package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	mw "mtg-chaos-draft/middleware"
	"mtg-chaos-draft/testhelper"
)

func newDraftHandler(t *testing.T) *handlers.DraftHandler {
	t.Helper()
	return handlers.NewDraftHandler(testhelper.Pool(t))
}

func make12Picks() []map[string]any {
	picks := make([]map[string]any, 12)
	for i := range picks {
		picks[i] = map[string]any{
			"setName":     "Zendikar Rising",
			"productType": "draft_booster",
			"marketPrice": 4.99,
		}
	}
	return picks
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestDraftHandler_Create_Valid(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "drafts")
	h := newDraftHandler(t)

	body, _ := json.Marshal(map[string]any{"picks": make12Picks()})
	req := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want %d, got %d — %s", http.StatusCreated, rr.Code, rr.Body)
	}
	var draft db.Draft
	if err := json.NewDecoder(rr.Body).Decode(&draft); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(draft.Picks) != 12 {
		t.Errorf("picks: want 12, got %d", len(draft.Picks))
	}
}

func TestDraftHandler_Create_BadJSON(t *testing.T) {
	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewReader([]byte("bad")))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDraftHandler_Create_WrongPickCount(t *testing.T) {
	h := newDraftHandler(t)
	picks := make12Picks()[:5]
	body, _ := json.Marshal(map[string]any{"picks": picks})
	req := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestDraftHandler_List_Empty(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "drafts")
	h := newDraftHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/drafts", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want %d, got %d", http.StatusOK, rr.Code)
	}
	var drafts []db.Draft
	if err := json.NewDecoder(rr.Body).Decode(&drafts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("want empty list, got %d", len(drafts))
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDraftHandler_Delete_BadID(t *testing.T) {
	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/drafts/bad", nil)
	req.SetPathValue("id", "notanumber")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDraftHandler_Delete_Valid(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "drafts")

	draft, err := db.CreateDraft(context.Background(), pool, []db.DraftPick{
		{SetName: "Zendikar Rising", ProductType: "draft_booster"},
	})
	if err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/drafts/", nil)
	req.SetPathValue("id", itoa(draft.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want %d, got %d — %s", http.StatusNoContent, rr.Code, rr.Body)
	}
}

// ── Approve ───────────────────────────────────────────────────────────────────

func TestDraftHandler_Approve_NoUser(t *testing.T) {
	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/1/approve", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Approve(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestDraftHandler_Approve_Valid(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "drafts")

	user, err := db.GetOrCreateUser(context.Background(), pool, "test-google-id", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	picks := []db.DraftPick{
		{SetName: "Zendikar Rising", ProductType: "draft_booster"},
	}
	draft, err := db.CreateDraft(context.Background(), pool, picks)
	if err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/", nil)
	req.SetPathValue("id", itoa(draft.ID))
	ctx := context.WithValue(req.Context(), mw.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Approve(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want %d, got %d — %s", http.StatusNoContent, rr.Code, rr.Body)
	}
}

func TestDraftHandler_Approve_BadID(t *testing.T) {
	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/bad/approve", nil)
	req.SetPathValue("id", "notanumber")
	ctx := context.WithValue(req.Context(), mw.UserContextKey, &db.User{ID: 1, Role: "admin"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Approve(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDraftHandler_Approve_NotFound(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "drafts")

	user, err := db.GetOrCreateUser(context.Background(), pool, "test-google-id", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	h := newDraftHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/99999/approve", nil)
	req.SetPathValue("id", "99999")
	ctx := context.WithValue(req.Context(), mw.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Approve(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("want %d, got %d", http.StatusNotFound, rr.Code)
	}
}

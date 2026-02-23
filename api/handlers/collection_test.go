package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	"mtg-chaos-draft/testhelper"
)

func newCollectionHandler(t *testing.T) *handlers.CollectionHandler {
	t.Helper()
	return handlers.NewCollectionHandler(testhelper.Pool(t))
}

func addTestPack(t *testing.T, mtgstocksID int, name, setName, productType string, qty int) *db.CollectionPack {
	t.Helper()
	pack, err := db.AddPack(context.Background(), testhelper.Pool(t),
		mtgstocksID, name, setName, productType, qty, 1.0, nil)
	if err != nil {
		t.Fatalf("seed pack: %v", err)
	}
	return pack
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestCollectionHandler_List_Empty(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/collection", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, rr.Code)
	}
	var packs []db.CollectionPack
	if err := json.NewDecoder(rr.Body).Decode(&packs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(packs) != 0 {
		t.Errorf("want empty list, got %d packs", len(packs))
	}
}

func TestCollectionHandler_List_WithPacks(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	addTestPack(t, 10101, "Alpha", "Limited Edition Alpha", "draft_booster", 2)
	h := newCollectionHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/collection", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, rr.Code)
	}
	var packs []db.CollectionPack
	_ = json.NewDecoder(rr.Body).Decode(&packs)
	if len(packs) != 1 {
		t.Errorf("want 1 pack, got %d", len(packs))
	}
}

// ── Add ──────────────────────────────────────────────────────────────────────

func TestCollectionHandler_Add_Valid(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)

	body, _ := json.Marshal(map[string]any{
		"mtgstocksId": 20202,
		"name":        "Beta",
		"setName":     "Limited Edition Beta",
		"productType": "draft_booster",
		"quantity":    1,
		"weight":      1.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/collection", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Add(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: want %d, got %d — body: %s", http.StatusCreated, rr.Code, rr.Body)
	}
	var pack db.CollectionPack
	if err := json.NewDecoder(rr.Body).Decode(&pack); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pack.Name != "Beta" {
		t.Errorf("name: want %q, got %q", "Beta", pack.Name)
	}
}

func TestCollectionHandler_Add_DefaultsQuantityAndWeight(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)

	body, _ := json.Marshal(map[string]any{
		"mtgstocksId": 20203,
		"name":        "Unlimited",
		"setName":     "Unlimited Edition",
		"productType": "draft_booster",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/collection", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Add(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: want %d, got %d", http.StatusCreated, rr.Code)
	}
	var pack db.CollectionPack
	_ = json.NewDecoder(rr.Body).Decode(&pack)
	if pack.Quantity != 1 {
		t.Errorf("default quantity: want 1, got %d", pack.Quantity)
	}
	if pack.Weight != 1.0 {
		t.Errorf("default weight: want 1.0, got %v", pack.Weight)
	}
}

func TestCollectionHandler_Add_BadJSON(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/collection", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	h.Add(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCollectionHandler_Add_MissingMTGStocksID(t *testing.T) {
	h := newCollectionHandler(t)
	body, _ := json.Marshal(map[string]any{"name": "foo", "productType": "draft_booster"})
	req := httptest.NewRequest(http.MethodPost, "/api/collection", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Add(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCollectionHandler_Add_MissingProductType(t *testing.T) {
	h := newCollectionHandler(t)
	body, _ := json.Marshal(map[string]any{"mtgstocksId": 1, "name": "foo"})
	req := httptest.NewRequest(http.MethodPost, "/api/collection", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Add(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestCollectionHandler_Update_Quantity(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	pack := addTestPack(t, 30301, "Revised", "Revised Edition", "draft_booster", 2)
	h := newCollectionHandler(t)

	body, _ := json.Marshal(map[string]any{"quantity": 5})
	req := httptest.NewRequest(http.MethodPut, "/api/collection/", bytes.NewReader(body))
	req.SetPathValue("id", itoa(pack.ID))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d — %s", http.StatusOK, rr.Code, rr.Body)
	}
	var updated db.CollectionPack
	_ = json.NewDecoder(rr.Body).Decode(&updated)
	if updated.Quantity != 5 {
		t.Errorf("quantity: want 5, got %d", updated.Quantity)
	}
}

func TestCollectionHandler_Update_BadID(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/collection/", bytes.NewReader([]byte("{}")))
	req.SetPathValue("id", "notanumber")
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCollectionHandler_Update_BadJSON(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/collection/", bytes.NewReader([]byte("bad")))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestCollectionHandler_Delete_Valid(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	pack := addTestPack(t, 40401, "Arabian Nights", "Arabian Nights", "draft_booster", 1)
	h := newCollectionHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/collection/", nil)
	req.SetPathValue("id", itoa(pack.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status: want %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestCollectionHandler_Delete_BadID(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/collection/", nil)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// ── LinkPrice ─────────────────────────────────────────────────────────────────

func TestCollectionHandler_LinkPrice_BadID(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/collection/bad/price", nil)
	req.SetPathValue("id", "notanumber")
	rr := httptest.NewRecorder()
	h.LinkPrice(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCollectionHandler_LinkPrice_BadJSON(t *testing.T) {
	h := newCollectionHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/collection/1/price", bytes.NewReader([]byte("bad")))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.LinkPrice(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCollectionHandler_LinkPrice_MissingMTGStocksID(t *testing.T) {
	h := newCollectionHandler(t)
	body, _ := json.Marshal(map[string]any{"mtgstocksId": 0})
	req := httptest.NewRequest(http.MethodPut, "/api/collection/1/price", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.LinkPrice(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/handlers"
	"mtg-chaos-draft/testhelper"
)

func newSelectHandler(t *testing.T) *handlers.SelectHandler {
	t.Helper()
	return handlers.NewSelectHandler(testhelper.Pool(t))
}

func TestSelectHandler_BadJSON(t *testing.T) {
	h := newSelectHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/select", bytes.NewReader([]byte("bad")))
	rr := httptest.NewRecorder()
	h.Select(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestSelectHandler_EmptyPackIDs(t *testing.T) {
	h := newSelectHandler(t)
	body, _ := json.Marshal(map[string]any{"packIds": []int{}})
	req := httptest.NewRequest(http.MethodPost, "/api/select", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Select(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestSelectHandler_NoMatchingPacks(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newSelectHandler(t)
	body, _ := json.Marshal(map[string]any{"packIds": []int{99999}})
	req := httptest.NewRequest(http.MethodPost, "/api/select", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Select(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestSelectHandler_Success(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	pack := addTestPack(t, 55501, "Zendikar Rising", "Zendikar Rising", "draft_booster", 3)
	h := newSelectHandler(t)

	body, _ := json.Marshal(map[string]any{"packIds": []int{pack.ID}})
	req := httptest.NewRequest(http.MethodPost, "/api/select", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Select(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want %d, got %d — %s", http.StatusOK, rr.Code, rr.Body)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gotID := int(resp["selectedId"].(float64))
	if gotID != pack.ID {
		t.Errorf("selectedId: want %d, got %d", pack.ID, gotID)
	}
}

func TestSelectHandler_MultiplePacks_ReturnsOne(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	p1 := addTestPack(t, 55601, "Ikoria", "Ikoria: Lair of Behemoths", "draft_booster", 2)
	p2 := addTestPack(t, 55602, "Strixhaven", "Strixhaven: School of Mages", "draft_booster", 2)
	h := newSelectHandler(t)

	body, _ := json.Marshal(map[string]any{"packIds": []int{p1.ID, p2.ID}})
	req := httptest.NewRequest(http.MethodPost, "/api/select", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Select(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want %d, got %d — %s", http.StatusOK, rr.Code, rr.Body)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	gotID := int(resp["selectedId"].(float64))
	if gotID != p1.ID && gotID != p2.ID {
		t.Errorf("selectedId %d is not one of the submitted pack IDs", gotID)
	}
}

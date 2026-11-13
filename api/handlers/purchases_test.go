package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	"mtg-chaos-draft/testhelper"
)

func postJSON(t *testing.T, h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func listPurchases(t *testing.T) []db.Purchase {
	t.Helper()
	list, err := db.ListPurchases(context.Background(), testhelper.Pool(t))
	if err != nil {
		t.Fatalf("ListPurchases: %v", err)
	}
	return list
}

func TestCollectionHandler_Add_WithPaidPrice(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)

	rr := postJSON(t, h.Add, "/api/collection", map[string]any{
		"mtgstocksId": 62001, "name": "FDN", "setName": "Foundations", "productType": "Play Booster",
		"quantity": 4, "marketPrice": 5.0, "paidPrice": 3.75, "purchasedOn": "2026-11-01",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d — %s", rr.Code, rr.Body)
	}
	list := listPurchases(t)
	if len(list) != 1 || list[0].UnitPrice != 3.75 || list[0].Quantity != 4 || list[0].PurchasedOn != "2026-11-01" {
		t.Errorf("unexpected purchases: %+v", list)
	}
}

func TestCollectionHandler_Add_WithoutPaidPriceLogsNothing(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)
	rr := postJSON(t, h.Add, "/api/collection", map[string]any{
		"mtgstocksId": 62002, "setName": "Foundations", "productType": "Collector Booster",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d", rr.Code)
	}
	if n := len(listPurchases(t)); n != 0 {
		t.Errorf("want no purchases, got %d", n)
	}
}

func TestCollectionHandler_Add_RejectsBadPurchase(t *testing.T) {
	h := newCollectionHandler(t)
	future := time.Now().AddDate(0, 0, 5).Format(time.DateOnly)
	for name, extra := range map[string]map[string]any{
		"negative price": {"paidPrice": -1},
		"bad date":       {"paidPrice": 1, "purchasedOn": "11/01/2026"},
		"future date":    {"paidPrice": 1, "purchasedOn": future},
	} {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"mtgstocksId": 62003, "productType": "Play Booster"}
			for k, v := range extra {
				body[k] = v
			}
			if rr := postJSON(t, h.Add, "/api/collection", body); rr.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rr.Code)
			}
		})
	}
}

func TestPurchaseHandler_CreateListDelete(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	pack := addTestPack(t, 62004, "OTJ", "Outlaws of Thunder Junction", "Play Booster", 2)
	h := handlers.NewPurchaseHandler(pool)

	rr := postJSON(t, h.Create, "/api/purchases", map[string]any{
		"packId": pack.ID, "quantity": 3, "paidPrice": 4.0, "addToStock": true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d — %s", rr.Code, rr.Body)
	}
	var created db.Purchase
	_ = json.NewDecoder(rr.Body).Decode(&created)
	packs, _ := db.GetPacksByIDs(context.Background(), pool, []int{pack.ID})
	if packs[0].Quantity != 5 {
		t.Errorf("addToStock: want quantity 5, got %d", packs[0].Quantity)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/purchases", nil)
	lr := httptest.NewRecorder()
	h.List(lr, req)
	var list []db.Purchase
	_ = json.NewDecoder(lr.Body).Decode(&list)
	if lr.Code != http.StatusOK || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list: %d %+v", lr.Code, list)
	}

	dreq := httptest.NewRequest(http.MethodDelete, "/api/purchases/x", nil)
	dreq.SetPathValue("id", itoa(created.ID))
	dr := httptest.NewRecorder()
	h.Delete(dr, dreq)
	if dr.Code != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", dr.Code)
	}
	if n := len(listPurchases(t)); n != 0 {
		t.Errorf("want 0 purchases after delete, got %d", n)
	}
}

func TestPurchaseHandler_List_Empty(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := handlers.NewPurchaseHandler(testhelper.Pool(t))
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/purchases", nil))
	if body := bytes.TrimSpace(rr.Body.Bytes()); string(body) != "[]" {
		t.Errorf("want [], got %s", body)
	}
}

func TestPurchaseHandler_Create_Invalid(t *testing.T) {
	h := handlers.NewPurchaseHandler(testhelper.Pool(t))
	cases := map[string]map[string]any{
		"missing price": {"packId": 1, "quantity": 1},
		"both prices":   {"packId": 1, "quantity": 1, "paidPrice": 1, "paidTotal": 1},
		"negative fees": {"packId": 1, "quantity": 1, "paidTotal": 1, "fees": -1},
		"missing pack":  {"quantity": 1, "paidPrice": 1},
		"zero quantity": {"packId": 1, "paidPrice": 1},
		"bad date":      {"packId": 1, "quantity": 1, "paidPrice": 1, "purchasedOn": "yesterday"},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if rr := postJSON(t, h.Create, "/api/purchases", body); rr.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rr.Code)
			}
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/api/purchases", bytes.NewReader([]byte("{")))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad json: want 400, got %d", rr.Code)
	}
}

func TestPurchaseHandler_Create_UnknownPack(t *testing.T) {
	h := handlers.NewPurchaseHandler(testhelper.Pool(t))
	rr := postJSON(t, h.Create, "/api/purchases", map[string]any{"packId": 999999, "quantity": 1, "paidPrice": 1})
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestPurchaseHandler_Delete_BadID(t *testing.T) {
	h := handlers.NewPurchaseHandler(testhelper.Pool(t))
	req := httptest.NewRequest(http.MethodDelete, "/api/purchases/x", nil)
	req.SetPathValue("id", "x")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestCollectionHandler_Add_WithPaidTotal(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := newCollectionHandler(t)
	rr := postJSON(t, h.Add, "/api/collection", map[string]any{
		"mtgstocksId": 62006, "setName": "Edge of Eternities", "productType": "Play Booster",
		"quantity": 30, "paidTotal": 132.18, "fees": 10.68,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d — %s", rr.Code, rr.Body)
	}
	list := listPurchases(t)
	// The total is kept exactly; the per-pack price is derived from it.
	if len(list) != 1 || list[0].TotalPrice != 132.18 || list[0].Fees != 10.68 || list[0].Quantity != 30 {
		t.Fatalf("unexpected purchases: %+v", list)
	}
	if got := list[0].UnitPrice; got < 4.4059 || got > 4.4061 {
		t.Errorf("unit price: want ~4.406, got %v", got)
	}
}

func TestCollectionHandler_Add_RejectsPriceAndTotal(t *testing.T) {
	h := newCollectionHandler(t)
	for name, body := range map[string]map[string]any{
		"both":            {"paidPrice": 1, "paidTotal": 30},
		"negative total":  {"paidTotal": -5},
		"fees over total": {"paidTotal": 5, "fees": 6},
	} {
		t.Run(name, func(t *testing.T) {
			body["mtgstocksId"] = 62007
			body["productType"] = "Play Booster"
			if rr := postJSON(t, h.Add, "/api/collection", body); rr.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rr.Code)
			}
		})
	}
}

func putJSON(t *testing.T, h http.HandlerFunc, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if s, ok := body.(string); ok {
		b = []byte(s)
	} else {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/purchases/"+id, bytes.NewReader(b))
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func TestPurchaseHandler_Update(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	pack := addTestPack(t, 62008, "SPM", "Marvel's Spider-Man", "Play Booster", 30)
	created, err := db.LogPurchase(context.Background(), pool, pack.ID, 30, 130, 0, "2026-11-01", false)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := handlers.NewPurchaseHandler(pool)

	rr := putJSON(t, h.Update, itoa(created.ID), map[string]any{"quantity": 30, "paidTotal": 132.18, "fees": 9.5, "purchasedOn": "2026-11-02"})
	if rr.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d — %s", rr.Code, rr.Body)
	}
	var got db.Purchase
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if got.TotalPrice != 132.18 || got.Fees != 9.5 || got.PurchasedOn != "2026-11-02" {
		t.Errorf("unexpected update: %+v", got)
	}

	// Per-pack price works too.
	rr = putJSON(t, h.Update, itoa(created.ID), map[string]any{"quantity": 2, "paidPrice": 4.5, "purchasedOn": "2026-11-02"})
	_ = json.NewDecoder(rr.Body).Decode(&got)
	if rr.Code != http.StatusOK || got.TotalPrice != 9 {
		t.Errorf("per-pack update: %d %+v", rr.Code, got)
	}
}

func TestPurchaseHandler_Update_Invalid(t *testing.T) {
	h := handlers.NewPurchaseHandler(testhelper.Pool(t))
	cases := map[string]struct {
		id   string
		body any
		want int
	}{
		"bad id":       {"x", map[string]any{}, http.StatusBadRequest},
		"bad json":     {"1", "{", http.StatusBadRequest},
		"no price":     {"1", map[string]any{"quantity": 1, "purchasedOn": "2026-11-01"}, http.StatusBadRequest},
		"no date":      {"1", map[string]any{"quantity": 1, "paidTotal": 1}, http.StatusBadRequest},
		"no quantity":  {"1", map[string]any{"paidTotal": 1, "purchasedOn": "2026-11-01"}, http.StatusBadRequest},
		"fees > total": {"1", map[string]any{"quantity": 1, "paidTotal": 1, "fees": 2, "purchasedOn": "2026-11-01"}, http.StatusBadRequest},
		"not found":    {"999999", map[string]any{"quantity": 1, "paidTotal": 1, "purchasedOn": "2026-11-01"}, http.StatusNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rr := putJSON(t, h.Update, tc.id, tc.body); rr.Code != tc.want {
				t.Errorf("want %d, got %d", tc.want, rr.Code)
			}
		})
	}
}

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
	"mtg-chaos-draft/testhelper"
)

func saleRequest(method, id string, body any) *http.Request {
	var b []byte
	if s, ok := body.(string); ok {
		b = []byte(s)
	} else {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, "/api/sales/"+id, bytes.NewReader(b))
	if id != "" {
		req.SetPathValue("id", id)
	}
	return req
}

func serve(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func TestSaleHandler_Flow(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	pack := addTestPack(t, 63001, "LCI", "Lost Caverns of Ixalan", "Play Booster", 10)
	h := handlers.NewSaleHandler(pool)

	rr := serve(h.Create, saleRequest(http.MethodPost, "", map[string]any{
		"packId": pack.ID, "quantity": 4, "saleTotal": 30.0, "fees": 3.0, "soldOn": "2026-11-02",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d — %s", rr.Code, rr.Body)
	}
	var sale db.Disposal
	_ = json.NewDecoder(rr.Body).Decode(&sale)
	if sale.Proceeds != 30 || sale.Fees != 3 || sale.Quantity != 4 {
		t.Errorf("unexpected sale: %+v", sale)
	}

	rr = serve(h.Update, saleRequest(http.MethodPut, itoa(sale.ID), map[string]any{
		"quantity": 5, "salePrice": 8.0, "soldOn": "2026-11-02",
	}))
	_ = json.NewDecoder(rr.Body).Decode(&sale)
	if rr.Code != http.StatusOK || sale.Proceeds != 40 || sale.Quantity != 5 {
		t.Errorf("update: %d %+v", rr.Code, sale)
	}

	lr := serve(h.List, httptest.NewRequest(http.MethodGet, "/api/disposals", nil))
	var list []db.Disposal
	_ = json.NewDecoder(lr.Body).Decode(&list)
	if lr.Code != http.StatusOK || len(list) != 1 {
		t.Errorf("list: %d %+v", lr.Code, list)
	}

	if rr := serve(h.Delete, saleRequest(http.MethodDelete, itoa(sale.ID), nil)); rr.Code != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", rr.Code)
	}
	packs, _ := db.GetPacksByIDs(context.Background(), pool, []int{pack.ID})
	if packs[0].Quantity != 10 {
		t.Errorf("stock after undoing the sale: want 10, got %d", packs[0].Quantity)
	}
}

func TestSaleHandler_List_Empty(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "collection_packs")
	h := handlers.NewSaleHandler(testhelper.Pool(t))
	rr := serve(h.List, httptest.NewRequest(http.MethodGet, "/api/disposals", nil))
	if body := bytes.TrimSpace(rr.Body.Bytes()); string(body) != "[]" {
		t.Errorf("want [], got %s", body)
	}
}

func TestSaleHandler_Errors(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	pack := addTestPack(t, 63002, "WOE", "Wilds of Eldraine", "Draft Booster", 1)
	h := handlers.NewSaleHandler(pool)

	create := map[string]struct {
		body any
		want int
	}{
		"bad json":       {"{", http.StatusBadRequest},
		"no quantity":    {map[string]any{"packId": pack.ID, "saleTotal": 5}, http.StatusBadRequest},
		"no price":       {map[string]any{"packId": pack.ID, "quantity": 1}, http.StatusBadRequest},
		"both prices":    {map[string]any{"packId": pack.ID, "quantity": 1, "salePrice": 1, "saleTotal": 1}, http.StatusBadRequest},
		"bad date":       {map[string]any{"packId": pack.ID, "quantity": 1, "saleTotal": 5, "soldOn": "soon"}, http.StatusBadRequest},
		"fees > total":   {map[string]any{"packId": pack.ID, "quantity": 1, "saleTotal": 5, "fees": 6}, http.StatusBadRequest},
		"no pack":        {map[string]any{"quantity": 1, "saleTotal": 5}, http.StatusBadRequest},
		"unknown pack":   {map[string]any{"packId": 999999, "quantity": 1, "saleTotal": 5}, http.StatusNotFound},
		"too many packs": {map[string]any{"packId": pack.ID, "quantity": 2, "saleTotal": 5}, http.StatusConflict},
	}
	for name, tc := range create {
		t.Run("create "+name, func(t *testing.T) {
			if rr := serve(h.Create, saleRequest(http.MethodPost, "", tc.body)); rr.Code != tc.want {
				t.Errorf("want %d, got %d — %s", tc.want, rr.Code, rr.Body)
			}
		})
	}

	update := map[string]struct {
		id   string
		body any
		want int
	}{
		"bad id":    {"x", map[string]any{}, http.StatusBadRequest},
		"bad json":  {"1", "{", http.StatusBadRequest},
		"no date":   {"1", map[string]any{"quantity": 1, "saleTotal": 5}, http.StatusBadRequest},
		"not found": {"999999", map[string]any{"quantity": 1, "saleTotal": 5, "soldOn": "2026-11-01"}, http.StatusNotFound},
	}
	for name, tc := range update {
		t.Run("update "+name, func(t *testing.T) {
			if rr := serve(h.Update, saleRequest(http.MethodPut, tc.id, tc.body)); rr.Code != tc.want {
				t.Errorf("want %d, got %d", tc.want, rr.Code)
			}
		})
	}

	if rr := serve(h.Delete, saleRequest(http.MethodDelete, "x", nil)); rr.Code != http.StatusBadRequest {
		t.Errorf("delete bad id: want 400, got %d", rr.Code)
	}
	if rr := serve(h.Delete, saleRequest(http.MethodDelete, "999999", nil)); rr.Code != http.StatusNotFound {
		t.Errorf("delete missing: want 404, got %d", rr.Code)
	}
}

func TestSaleHandler_SetDraftCollected(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "drafts", "collection_packs", "users")
	ctx := context.Background()
	pack := addTestPack(t, 63003, "MKM", "Murders at Karlov Manor", "Play Booster", 10)
	price := 5.0
	id := pack.ID
	draft, err := db.CreateDraft(ctx, pool, 2, []db.DraftPick{
		{PackID: &id, SetName: "MKM", ProductType: "Play Booster", MarketPrice: &price},
		{PackID: &id, SetName: "MKM", ProductType: "Play Booster", MarketPrice: &price},
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := db.GetOrCreateUser(ctx, pool, "g-sales", "s@example.com", "S", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	h := handlers.NewSaleHandler(pool)
	put := func(id string, body any) *httptest.ResponseRecorder {
		var b []byte
		if s, ok := body.(string); ok {
			b = []byte(s)
		} else {
			b, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(http.MethodPut, "/api/drafts/"+id+"/collected", bytes.NewReader(b))
		req.SetPathValue("id", id)
		return serve(h.SetDraftCollected, req)
	}

	rr := put(itoa(draft.ID), map[string]any{"amount": 6})
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", rr.Code, rr.Body)
	}
	var list []db.Disposal
	_ = json.NewDecoder(rr.Body).Decode(&list)
	if len(list) != 1 || list[0].Proceeds != 6 || list[0].MarketValue == nil || *list[0].MarketValue != 10 {
		t.Errorf("unexpected: %+v", list)
	}

	for name, tc := range map[string]struct {
		id   string
		body any
		want int
	}{
		"bad id":    {"x", map[string]any{"amount": 1}, http.StatusBadRequest},
		"bad json":  {"1", "{", http.StatusBadRequest},
		"no amount": {"1", map[string]any{}, http.StatusBadRequest},
		"negative":  {"1", map[string]any{"amount": -1}, http.StatusBadRequest},
		"unknown":   {"999999", map[string]any{"amount": 1}, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			if rr := put(tc.id, tc.body); rr.Code != tc.want {
				t.Errorf("want %d, got %d", tc.want, rr.Code)
			}
		})
	}
}

package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	"mtg-chaos-draft/testhelper"
)

func newSettingsHandler(t *testing.T) *handlers.SettingsHandler {
	t.Helper()
	return handlers.NewSettingsHandler(testhelper.Pool(t))
}

func TestSettingsHandler_Get_ReturnsDefaults(t *testing.T) {
	h := newSettingsHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d", http.StatusOK, rr.Code)
	}
	var s db.WeightSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.PriceSensitivity != 0.5 {
		t.Errorf("price sensitivity: want 0.5, got %v", s.PriceSensitivity)
	}
}

func TestSettingsHandler_Update_ClampsValues(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "pack_weight_overrides")
	h := newSettingsHandler(t)

	body, _ := json.Marshal(map[string]any{
		"priceSensitivity":    1.5,  // should clamp to 1.0
		"scarcitySensitivity": -0.2, // should clamp to 0.0
		"priceCap":            -10,  // should clamp to 0
		"priceFloor":          5.0,
		"quantityCap":         8,
		"packWeights":         map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want %d, got %d — %s", http.StatusOK, rr.Code, rr.Body)
	}
	var s db.WeightSettings
	_ = json.NewDecoder(rr.Body).Decode(&s)
	if s.PriceSensitivity != 1.0 {
		t.Errorf("price sensitivity clamped: want 1.0, got %v", s.PriceSensitivity)
	}
	if s.ScaricitySensitivity != 0.0 {
		t.Errorf("scarcity sensitivity clamped: want 0.0, got %v", s.ScaricitySensitivity)
	}
	if s.PriceCap != 0 {
		t.Errorf("price cap clamped: want 0, got %v", s.PriceCap)
	}
	if s.PriceFloor != 5.0 {
		t.Errorf("price floor: want 5.0, got %v", s.PriceFloor)
	}
	if s.QuantityCap != 8 {
		t.Errorf("quantity cap: want 8, got %d", s.QuantityCap)
	}
}

func TestSettingsHandler_Update_BadJSON(t *testing.T) {
	h := newSettingsHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader([]byte("bad")))
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestSettingsHandler_Update_NilPackWeightsDefaultsToEmpty(t *testing.T) {
	testhelper.Truncate(t, testhelper.Pool(t), "pack_weight_overrides")
	h := newSettingsHandler(t)

	body, _ := json.Marshal(map[string]any{
		"priceSensitivity":    0.5,
		"scarcitySensitivity": 0.5,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want %d, got %d — %s", http.StatusOK, rr.Code, rr.Body)
	}
	var s db.WeightSettings
	_ = json.NewDecoder(rr.Body).Decode(&s)
	if s.PackWeights == nil {
		t.Error("pack weights should default to empty map, not nil")
	}
}

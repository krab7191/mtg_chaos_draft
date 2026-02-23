package db_test

import (
	"context"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestGetWeightSettings_ReturnsDefaults(t *testing.T) {
	pool := testhelper.Pool(t)
	s, err := db.GetWeightSettings(context.Background(), pool)
	if err != nil {
		t.Fatalf("GetWeightSettings: %v", err)
	}
	if s.PriceSensitivity != 0.5 {
		t.Errorf("price sensitivity: want 0.5, got %v", s.PriceSensitivity)
	}
	if s.ScaricitySensitivity != 0.5 {
		t.Errorf("scarcity sensitivity: want 0.5, got %v", s.ScaricitySensitivity)
	}
	if s.PackWeights == nil {
		t.Error("pack weights map should not be nil")
	}
}

func TestUpdateWeightSettings(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "pack_weight_overrides")
	ctx := context.Background()

	// Reset settings to defaults first
	orig, _ := db.GetWeightSettings(ctx, pool)
	orig.PriceCap = 50.0
	orig.PriceFloor = 5.0
	orig.QuantityCap = 10

	updated, err := db.UpdateWeightSettings(ctx, pool, orig)
	if err != nil {
		t.Fatalf("UpdateWeightSettings: %v", err)
	}
	if updated.PriceCap != 50.0 {
		t.Errorf("price cap: want 50.0, got %v", updated.PriceCap)
	}
	if updated.PriceFloor != 5.0 {
		t.Errorf("price floor: want 5.0, got %v", updated.PriceFloor)
	}
	if updated.QuantityCap != 10 {
		t.Errorf("quantity cap: want 10, got %d", updated.QuantityCap)
	}
}

func TestUpdateWeightSettings_PackWeights(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs", "pack_weight_overrides")
	ctx := context.Background()

	pack, _ := db.AddPack(ctx, pool, 99901, "Pack A", "Pack A", "draft_booster", 1, 1.0, nil)

	settings, _ := db.GetWeightSettings(ctx, pool)
	settings.PackWeights = map[int]float64{pack.ID: 2.5}

	updated, err := db.UpdateWeightSettings(ctx, pool, settings)
	if err != nil {
		t.Fatalf("UpdateWeightSettings with pack weights: %v", err)
	}
	if mult, ok := updated.PackWeights[pack.ID]; !ok || mult != 2.5 {
		t.Errorf("pack weight: want 2.5 for pack %d, got %v (ok=%v)", pack.ID, mult, ok)
	}
}

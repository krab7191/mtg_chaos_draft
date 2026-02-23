package handlers

import (
	"math"
	"testing"

	"mtg-chaos-draft/db"
)

func fptr(v float64) *float64 { return &v }

func defaultSettings() *db.WeightSettings {
	return &db.WeightSettings{PackWeights: map[int]float64{}}
}

// ── effectiveWeights ──────────────────────────────────────────────────────────

func TestEffectiveWeights_EqualPacks(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(10.0), Quantity: 1},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 1},
	}
	weights := effectiveWeights(packs, defaultSettings())
	if len(weights) != 2 {
		t.Fatalf("want 2 weights, got %d", len(weights))
	}
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("equal packs should have equal weights, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_ZeroQuantityGetsZeroWeight(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(10.0), Quantity: 0},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 1},
	}
	weights := effectiveWeights(packs, defaultSettings())
	if weights[0] != 0 {
		t.Errorf("zero-quantity pack should have zero weight, got %v", weights[0])
	}
	if weights[1] <= 0 {
		t.Errorf("non-zero-quantity pack should have positive weight, got %v", weights[1])
	}
}

func TestEffectiveWeights_PriceCapClamps(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(100.0), Quantity: 1},
		{ID: 2, MarketPrice: fptr(200.0), Quantity: 1},
	}
	settings := &db.WeightSettings{PriceCap: 100.0, PackWeights: map[int]float64{}}
	weights := effectiveWeights(packs, settings)
	// Both capped to $100, so price odds are equal → weights should be equal
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("price-capped packs should have equal weights, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_PriceFloorRaisesChapPacks(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(1.0), Quantity: 1},
		{ID: 2, MarketPrice: fptr(1.0), Quantity: 1},
	}
	settings := &db.WeightSettings{PriceFloor: 10.0, PackWeights: map[int]float64{}}
	weights := effectiveWeights(packs, settings)
	// Both floored to $10, so equal weights
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("floored packs should have equal weights, got %v vs %v", weights[0], weights[1])
	}
	for i, w := range weights {
		if w <= 0 {
			t.Errorf("weight[%d] should be positive after floor, got %v", i, w)
		}
	}
}

func TestEffectiveWeights_QuantityCapTreatsHighQtyAsCapValue(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(10.0), Quantity: 100},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 200},
	}
	settings := &db.WeightSettings{QuantityCap: 10, PackWeights: map[int]float64{}}
	weights := effectiveWeights(packs, settings)
	// Both capped at qty=10, so equal scarcity odds → equal weights
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("qty-capped packs should have equal weights, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_PackWeightMultiplierScalesWeight(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(10.0), Quantity: 1},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 1},
	}
	settings := &db.WeightSettings{PackWeights: map[int]float64{1: 2.0}}
	weights := effectiveWeights(packs, settings)
	// Pack 1 has a 2× multiplier, so it should have double the weight
	if math.Abs(weights[0]-2*weights[1]) > 1e-9 {
		t.Errorf("pack with 2× multiplier should have double weight, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_NilPriceUsesAverage(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: nil, Quantity: 1},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 1},
	}
	weights := effectiveWeights(packs, defaultSettings())
	// Nil-price pack uses average ($10), so both have same price weight
	// and same quantity → equal weights
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("unpriced pack should use average price, want equal weights, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_AllNilPricesFallbackToOne(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: nil, Quantity: 1},
		{ID: 2, MarketPrice: nil, Quantity: 2},
	}
	weights := effectiveWeights(packs, defaultSettings())
	if len(weights) != 2 {
		t.Fatalf("want 2 weights, got %d", len(weights))
	}
	for i, w := range weights {
		if w < 0 {
			t.Errorf("weight[%d] should be non-negative, got %v", i, w)
		}
	}
	// Pack 2 has more quantity so it should be less scarce → lower weight
	if weights[1] >= weights[0] {
		t.Errorf("higher-qty pack should have lower scarcity weight, got %v >= %v", weights[1], weights[0])
	}
}

func TestEffectiveWeights_ZeroMultiplierIsIgnored(t *testing.T) {
	// The code only applies a PackWeight multiplier when mult > 0.
	// A zero multiplier entry is ignored, so the pack keeps its base weight.
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(10.0), Quantity: 1},
		{ID: 2, MarketPrice: fptr(10.0), Quantity: 1},
	}
	settings := &db.WeightSettings{PackWeights: map[int]float64{1: 0}}
	weights := effectiveWeights(packs, settings)
	// Both packs should have equal weight since the zero multiplier is not applied
	if math.Abs(weights[0]-weights[1]) > 1e-9 {
		t.Errorf("zero-multiplier is ignored: want equal weights, got %v vs %v", weights[0], weights[1])
	}
}

func TestEffectiveWeights_AllWeightsNonNegative(t *testing.T) {
	packs := []db.CollectionPack{
		{ID: 1, MarketPrice: fptr(5.0), Quantity: 2},
		{ID: 2, MarketPrice: fptr(20.0), Quantity: 1},
		{ID: 3, MarketPrice: fptr(10.0), Quantity: 3},
	}
	weights := effectiveWeights(packs, defaultSettings())
	for i, w := range weights {
		if w < 0 {
			t.Errorf("weight[%d] = %v, want >= 0", i, w)
		}
	}
}

// ── weightedRandom ────────────────────────────────────────────────────────────

func TestWeightedRandom_SinglePack(t *testing.T) {
	packs := []db.CollectionPack{{ID: 42}}
	result := weightedRandom(packs, []float64{1.0})
	if result.ID != 42 {
		t.Errorf("want ID 42, got %d", result.ID)
	}
}

func TestWeightedRandom_ZeroWeightNeverSelected(t *testing.T) {
	packs := []db.CollectionPack{{ID: 1}, {ID: 2}}
	weights := []float64{0.0, 1.0}
	for range 2000 {
		result := weightedRandom(packs, weights)
		if result.ID == 1 {
			t.Fatal("zero-weight pack was selected")
		}
	}
}

func TestWeightedRandom_DistributionMatchesWeights(t *testing.T) {
	// Pack 1 has 3× the weight → should be selected ~75% of the time
	packs := []db.CollectionPack{{ID: 1}, {ID: 2}}
	weights := []float64{3.0, 1.0}
	counts := [2]int{}
	const trials = 20_000
	for range trials {
		result := weightedRandom(packs, weights)
		counts[result.ID-1]++
	}
	ratio := float64(counts[0]) / trials
	if ratio < 0.70 || ratio > 0.80 {
		t.Errorf("expected pack1 ~75%% of %d trials, got %.1f%%", trials, ratio*100)
	}
}

func TestWeightedRandom_FallsBackToLastPack(t *testing.T) {
	// If due to floating-point the loop exhausts all weights, it returns the last pack
	packs := []db.CollectionPack{{ID: 1}, {ID: 2}, {ID: 3}}
	// Very small total weight — this exercises the fallback path
	weights := []float64{0.0, 0.0, 1.0}
	result := weightedRandom(packs, weights)
	if result.ID != 3 {
		t.Errorf("want ID 3 (only non-zero weight), got %d", result.ID)
	}
}

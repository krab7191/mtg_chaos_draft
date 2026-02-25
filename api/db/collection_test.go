package db_test

import (
	"context"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func fptr(v float64) *float64 { return &v }

func TestListCollection_Empty(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")

	packs, err := db.ListCollection(context.Background(), pool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(packs) != 0 {
		t.Errorf("want 0 packs, got %d", len(packs))
	}
}

func TestAddPack_AndList(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	pack, err := db.AddPack(ctx, pool, 11111, "Dominaria", "Dominaria United", "", "draft_booster", 3, 1.5, fptr(12.99), 15)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}
	if pack.Name != "Dominaria" {
		t.Errorf("name: want %q, got %q", "Dominaria", pack.Name)
	}
	if pack.Quantity != 3 {
		t.Errorf("quantity: want 3, got %d", pack.Quantity)
	}
	if pack.MarketPrice == nil || *pack.MarketPrice != 12.99 {
		t.Errorf("market price: want 12.99, got %v", pack.MarketPrice)
	}

	packs, err := db.ListCollection(ctx, pool)
	if err != nil {
		t.Fatalf("ListCollection: %v", err)
	}
	if len(packs) != 1 || packs[0].ID != pack.ID {
		t.Errorf("want 1 pack with id %d, got %v", pack.ID, packs)
	}
}

func TestAddPack_ConflictIncrementsQuantity(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	_, err := db.AddPack(ctx, pool, 22222, "Kamigawa", "Kamigawa: Neon Dynasty", "", "draft_booster", 2, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("first AddPack: %v", err)
	}
	pack, err := db.AddPack(ctx, pool, 22222, "Kamigawa", "Kamigawa: Neon Dynasty", "", "draft_booster", 3, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("second AddPack (conflict): %v", err)
	}
	if pack.Quantity != 5 {
		t.Errorf("quantity after conflict: want 5, got %d", pack.Quantity)
	}
}

func TestAddPack_NilPrice(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")

	pack, err := db.AddPack(context.Background(), pool, 33333, "Innistrad", "Innistrad: Crimson Vow", "", "set_booster", 1, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}
	if pack.MarketPrice != nil {
		t.Errorf("want nil market price, got %v", pack.MarketPrice)
	}
}

func TestUpdatePack_Quantity(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	orig, err := db.AddPack(ctx, pool, 44444, "Strixhaven", "Strixhaven", "", "draft_booster", 4, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}
	qty := 7
	updated, err := db.UpdatePack(ctx, pool, orig.ID, &qty, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePack: %v", err)
	}
	if updated.Quantity != 7 {
		t.Errorf("quantity: want 7, got %d", updated.Quantity)
	}
}

func TestUpdatePack_Notes(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	orig, err := db.AddPack(ctx, pool, 55555, "Zendikar", "Zendikar Rising", "", "play_booster", 1, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}
	notes := "gift for friday night"
	updated, err := db.UpdatePack(ctx, pool, orig.ID, nil, nil, &notes, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePack: %v", err)
	}
	if updated.Notes != notes {
		t.Errorf("notes: want %q, got %q", notes, updated.Notes)
	}
}

func TestDeletePack(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	pack, err := db.AddPack(ctx, pool, 66666, "Theros", "Theros Beyond Death", "", "collector_booster", 1, 1.0, nil, 15)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}
	if err := db.DeletePack(ctx, pool, pack.ID); err != nil {
		t.Fatalf("DeletePack: %v", err)
	}
	packs, _ := db.ListCollection(ctx, pool)
	if len(packs) != 0 {
		t.Errorf("want 0 packs after delete, got %d", len(packs))
	}
}

func TestGetPacksByIDs(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	p1, _ := db.AddPack(ctx, pool, 77771, "Set A", "Set A", "", "draft_booster", 1, 1.0, nil, 15)
	p2, _ := db.AddPack(ctx, pool, 77772, "Set B", "Set B", "", "draft_booster", 1, 1.0, nil, 15)
	_, _ = db.AddPack(ctx, pool, 77773, "Set C", "Set C", "", "draft_booster", 1, 1.0, nil, 15)

	packs, err := db.GetPacksByIDs(ctx, pool, []int{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("GetPacksByIDs: %v", err)
	}
	if len(packs) != 2 {
		t.Errorf("want 2 packs, got %d", len(packs))
	}
}

func TestGetPacksByIDs_Empty(t *testing.T) {
	pool := testhelper.Pool(t)
	packs, err := db.GetPacksByIDs(context.Background(), pool, []int{})
	if err != nil {
		t.Fatalf("GetPacksByIDs: %v", err)
	}
	if len(packs) != 0 {
		t.Errorf("want 0 packs, got %d", len(packs))
	}
}

func TestBulkUpdatePrices(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	pack, _ := db.AddPack(ctx, pool, 88888, "Eldraine", "Throne of Eldraine", "", "draft_booster", 1, 1.0, nil, 15)

	err := db.BulkUpdatePrices(ctx, pool, map[int]float64{88888: 24.99})
	if err != nil {
		t.Fatalf("BulkUpdatePrices: %v", err)
	}
	packs, _ := db.GetPacksByIDs(ctx, pool, []int{pack.ID})
	if len(packs) == 0 || packs[0].MarketPrice == nil || *packs[0].MarketPrice != 24.99 {
		t.Errorf("price not updated, got: %v", packs)
	}
}

func TestBulkUpdatePrices_EmptyMap(t *testing.T) {
	pool := testhelper.Pool(t)
	if err := db.BulkUpdatePrices(context.Background(), pool, map[int]float64{}); err != nil {
		t.Errorf("BulkUpdatePrices with empty map: %v", err)
	}
}

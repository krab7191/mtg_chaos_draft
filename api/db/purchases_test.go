package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestAddPackWithPurchase_RecordsPurchase(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	pack, p, err := db.AddPackWithPurchase(ctx, pool, 61001, "MH3", "Modern Horizons 3", "MH3", "Play Booster", 6, 1.0, fptr(7.5), 14, 31.5, 0, "2026-10-01")
	if err != nil {
		t.Fatalf("AddPackWithPurchase: %v", err)
	}
	if pack.Quantity != 6 {
		t.Errorf("quantity: want 6, got %d", pack.Quantity)
	}
	if p.PackID != pack.ID || p.Quantity != 6 || p.TotalPrice != 31.5 || p.UnitPrice != 5.25 {
		t.Errorf("unexpected purchase: %+v", p)
	}
	if p.PurchasedOn != "2026-10-01" || p.SetName != "Modern Horizons 3" || p.ProductType != "Play Booster" {
		t.Errorf("unexpected purchase details: %+v", p)
	}

	// Buying more of the same pack merges stock and adds a second purchase.
	pack, _, err = db.AddPackWithPurchase(ctx, pool, 61001, "MH3", "Modern Horizons 3", "MH3", "Play Booster", 2, 1.0, nil, 14, 6, 0, "")
	if err != nil {
		t.Fatalf("second AddPackWithPurchase: %v", err)
	}
	if pack.Quantity != 8 {
		t.Errorf("merged quantity: want 8, got %d", pack.Quantity)
	}
	list, err := db.ListPurchases(ctx, pool)
	if err != nil {
		t.Fatalf("ListPurchases: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 purchases, got %d", len(list))
	}
	// Newest date first; an empty date means today.
	if list[0].PurchasedOn != time.Now().Format(time.DateOnly) {
		t.Errorf("first listed purchase: %+v", list[0])
	}
	if list[0].CurrentPrice == nil || *list[0].CurrentPrice != 7.5 {
		t.Errorf("current price: want 7.5, got %v", list[0].CurrentPrice)
	}
}

func TestAddPackWithPurchase_RollsBackOnBadPurchase(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()

	// Negative prices violate the table's CHECK, so the pack must not be added either.
	if _, _, err := db.AddPackWithPurchase(ctx, pool, 61002, "X", "X", "", "Booster", 1, 1.0, nil, 15, -1, 0, ""); err == nil {
		t.Fatal("want error for negative price")
	}
	packs, _ := db.ListCollection(ctx, pool)
	if len(packs) != 0 {
		t.Errorf("pack should not be added when the purchase fails, got %d", len(packs))
	}
}

func TestLogPurchase(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()
	pack, err := db.AddPack(ctx, pool, 61003, "DSK", "Duskmourn", "DSK", "Play Booster", 3, 1.0, nil, 14)
	if err != nil {
		t.Fatalf("AddPack: %v", err)
	}

	if _, err := db.LogPurchase(ctx, pool, pack.ID, 3, 4, 0, "2026-09-01", false); err != nil {
		t.Fatalf("LogPurchase (backfill): %v", err)
	}
	if _, err := db.LogPurchase(ctx, pool, pack.ID, 2, 4.5, 0, "", true); err != nil {
		t.Fatalf("LogPurchase (restock): %v", err)
	}
	packs, _ := db.GetPacksByIDs(ctx, pool, []int{pack.ID})
	if packs[0].Quantity != 5 {
		t.Errorf("quantity: want 5 (3 + 2 restocked), got %d", packs[0].Quantity)
	}

	if _, err := db.LogPurchase(ctx, pool, 999999, 1, 4, 0, "", true); !errors.Is(err, db.ErrPackNotFound) {
		t.Errorf("restock missing pack: want ErrPackNotFound, got %v", err)
	}
	if _, err := db.LogPurchase(ctx, pool, 999999, 1, 4, 0, "", false); !errors.Is(err, db.ErrPackNotFound) {
		t.Errorf("log missing pack: want ErrPackNotFound, got %v", err)
	}
}

func TestDeletePurchase_AndCascade(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()
	pack, p, err := db.AddPackWithPurchase(ctx, pool, 61004, "BLB", "Bloomburrow", "BLB", "Play Booster", 1, 1.0, nil, 14, 4, 0, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, _ = db.LogPurchase(ctx, pool, pack.ID, 1, 4, 0, "", false)

	if err := db.DeletePurchase(ctx, pool, p.ID); err != nil {
		t.Fatalf("DeletePurchase: %v", err)
	}
	list, _ := db.ListPurchases(ctx, pool)
	if len(list) != 1 {
		t.Fatalf("want 1 purchase left, got %d", len(list))
	}
	packs, _ := db.GetPacksByIDs(ctx, pool, []int{pack.ID})
	if packs[0].Quantity != 1 {
		t.Errorf("deleting a purchase must not change stock, got %d", packs[0].Quantity)
	}

	// Removing the pack removes its purchases.
	if err := db.DeletePack(ctx, pool, pack.ID); err != nil {
		t.Fatalf("DeletePack: %v", err)
	}
	list, _ = db.ListPurchases(ctx, pool)
	if len(list) != 0 {
		t.Errorf("want purchases removed with their pack, got %d", len(list))
	}
}

func TestUpdatePurchase(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "collection_packs")
	ctx := context.Background()
	today := time.Now().Format(time.DateOnly)

	pack, p, err := db.AddPackWithPurchase(ctx, pool, 61005, "TDM", "Tarkir: Dragonstorm", "TDM", "Play Booster", 30, 1.0, fptr(5), 14, 130, 0, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	u, err := db.UpdatePurchase(ctx, pool, p.ID, 36, 150, 12.5, "2026-01-15")
	if err != nil {
		t.Fatalf("UpdatePurchase: %v", err)
	}
	if u.Quantity != 36 || u.TotalPrice != 150 || u.Fees != 12.5 || u.PurchasedOn != "2026-01-15" {
		t.Errorf("unexpected update: %+v", u)
	}
	if u.CurrentPrice == nil || *u.CurrentPrice != 5 {
		t.Errorf("current price: want 5, got %v", u.CurrentPrice)
	}

	packs, _ := db.GetPacksByIDs(ctx, pool, []int{pack.ID})
	if packs[0].Quantity != 30 {
		t.Errorf("editing a purchase must not change stock, got %d", packs[0].Quantity)
	}

	if _, err := db.UpdatePurchase(ctx, pool, 999999, 1, 1, 0, today); !errors.Is(err, db.ErrPurchaseNotFound) {
		t.Errorf("missing purchase: want ErrPurchaseNotFound, got %v", err)
	}
}

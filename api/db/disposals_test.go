package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestApproveDraft_RecordsDisposals(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	normal := seedDraftPack(t, "Normal", 5, 15)    // $7 a pick, 1 pack per slot
	epilogue := seedDraftPack(t, "Epilogue", 7, 5) // 3 packs per slot

	picks := picksOf(normal, normal, epilogue)
	slot := 21.0 // what a player paid for the Epilogue slot
	picks[2].MarketPrice = &slot
	draft, err := db.CreateDraft(ctx, pool, 4, picks)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, seedAdmin(t).ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	list, err := db.ListDisposals(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	byPack := map[int]db.Disposal{}
	for _, d := range list {
		byPack[d.PackID] = d
	}
	if d := byPack[normal.ID]; d.Kind != "draft" || d.Quantity != 2 || d.Proceeds != 14 || d.DraftID == nil || *d.DraftID != draft.ID {
		t.Errorf("normal: %+v", d)
	}
	if d := byPack[epilogue.ID]; d.Quantity != 3 || d.Proceeds != 21 {
		t.Errorf("epilogue: want 3 packs for $21, got %+v", d)
	}
	if d := byPack[normal.ID]; d.DisposedOn != time.Now().Format(time.DateOnly) || d.SetName != "Normal" {
		t.Errorf("details: %+v", d)
	}

	// Only $8.75 was collected (players kept few cards): split by market value, 14 : 21.
	got, err := db.SetDraftCollected(ctx, pool, draft.ID, 8.75)
	if err != nil {
		t.Fatalf("SetDraftCollected: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d", len(got))
	}
	for _, d := range got {
		want := map[int]float64{normal.ID: 3.5, epilogue.ID: 5.25}[d.PackID]
		if d.Proceeds < want-0.001 || d.Proceeds > want+0.001 {
			t.Errorf("%s: want proceeds %.2f, got %v", d.SetName, want, d.Proceeds)
		}
		if d.MarketValue == nil || *d.MarketValue == d.Proceeds {
			t.Errorf("%s: market value should be kept, got %v", d.SetName, d.MarketValue)
		}
	}
	if _, err := db.SetDraftCollected(ctx, pool, 999999, 10); !errors.Is(err, db.ErrDraftNotRecorded) {
		t.Errorf("unknown draft: want ErrDraftNotRecorded, got %v", err)
	}

	// Deleting the draft keeps the record: the packs were still opened.
	if err := db.DeleteDraft(ctx, pool, draft.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = db.ListDisposals(ctx, pool)
	if len(list) != 2 || list[0].DraftID != nil {
		t.Errorf("after draft delete: %+v", list)
	}
	// A sale can't be edited or deleted through a draft disposal.
	if _, err := db.UpdateSale(ctx, pool, list[0].ID, 1, 1, 0, "2026-11-01"); !errors.Is(err, db.ErrSaleNotFound) {
		t.Errorf("update draft disposal as sale: want ErrSaleNotFound, got %v", err)
	}
	if err := db.DeleteSale(ctx, pool, list[0].ID); !errors.Is(err, db.ErrSaleNotFound) {
		t.Errorf("delete draft disposal as sale: want ErrSaleNotFound, got %v", err)
	}
}

func stockOf(t *testing.T, id int) int {
	t.Helper()
	packs, err := db.GetPacksByIDs(context.Background(), testhelper.Pool(t), []int{id})
	if err != nil || len(packs) != 1 {
		t.Fatalf("get pack %d: %v", id, err)
	}
	return packs[0].Quantity
}

func TestSales_RecordUpdateDelete(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	pack := seedDraftPack(t, "Sold", 10, 15)

	sale, err := db.RecordSale(ctx, pool, pack.ID, 4, 30, 3.5, "2026-11-02")
	if err != nil {
		t.Fatalf("RecordSale: %v", err)
	}
	if sale.Kind != "sale" || sale.Quantity != 4 || sale.Proceeds != 30 || sale.Fees != 3.5 || sale.DisposedOn != "2026-11-02" {
		t.Errorf("unexpected sale: %+v", sale)
	}
	if got := stockOf(t, pack.ID); got != 6 {
		t.Errorf("stock after sale: want 6, got %d", got)
	}

	// More packs sold: the extra come out of stock.
	sale, err = db.UpdateSale(ctx, pool, sale.ID, 6, 45, 0, "2026-11-03")
	if err != nil {
		t.Fatalf("UpdateSale: %v", err)
	}
	if sale.Quantity != 6 || sale.Proceeds != 45 || sale.DisposedOn != "2026-11-03" || stockOf(t, pack.ID) != 4 {
		t.Errorf("after increase: %+v, stock %d", sale, stockOf(t, pack.ID))
	}
	// Fewer: the difference goes back.
	if _, err := db.UpdateSale(ctx, pool, sale.ID, 2, 15, 0, "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	if got := stockOf(t, pack.ID); got != 8 {
		t.Errorf("after decrease: want 8, got %d", got)
	}
	// Same quantity: stock untouched.
	if _, err := db.UpdateSale(ctx, pool, sale.ID, 2, 16, 0, "2026-11-03"); err != nil || stockOf(t, pack.ID) != 8 {
		t.Errorf("same quantity: err %v, stock %d", err, stockOf(t, pack.ID))
	}
	// Can't sell more than is in stock.
	if _, err := db.UpdateSale(ctx, pool, sale.ID, 20, 16, 0, "2026-11-03"); !errors.Is(err, db.ErrNotEnoughStock) {
		t.Errorf("oversell on update: want ErrNotEnoughStock, got %v", err)
	}

	if err := db.DeleteSale(ctx, pool, sale.ID); err != nil {
		t.Fatalf("DeleteSale: %v", err)
	}
	if got := stockOf(t, pack.ID); got != 10 {
		t.Errorf("after undoing the sale: want 10, got %d", got)
	}
	if list, _ := db.ListDisposals(ctx, pool); len(list) != 0 {
		t.Errorf("want no disposals, got %d", len(list))
	}
}

func TestRecordSale_Errors(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	pack := seedDraftPack(t, "Few", 2, 15)

	if _, err := db.RecordSale(ctx, pool, pack.ID, 3, 10, 0, ""); !errors.Is(err, db.ErrNotEnoughStock) {
		t.Errorf("oversell: want ErrNotEnoughStock, got %v", err)
	}
	if _, err := db.RecordSale(ctx, pool, 999999, 1, 10, 0, ""); !errors.Is(err, db.ErrPackNotFound) {
		t.Errorf("missing pack: want ErrPackNotFound, got %v", err)
	}
	if _, err := db.UpdateSale(ctx, pool, 999999, 1, 10, 0, "2026-11-01"); !errors.Is(err, db.ErrSaleNotFound) {
		t.Errorf("update missing: want ErrSaleNotFound, got %v", err)
	}
	if got := stockOf(t, pack.ID); got != 2 {
		t.Errorf("failed sales must not change stock, got %d", got)
	}
	// Fees above the sale price violate the table's CHECK and roll back.
	if _, err := db.RecordSale(ctx, pool, pack.ID, 1, 5, 6, ""); err == nil {
		t.Error("want error for fees > proceeds")
	}
	if got := stockOf(t, pack.ID); got != 2 {
		t.Errorf("rolled-back sale must not change stock, got %d", got)
	}
}

func TestSetDraftCollected_SplitsByPacksWithoutPrices(t *testing.T) {
	cleanDraftTables(t)
	ctx := context.Background()
	pool := testhelper.Pool(t)
	a := seedDraftPack(t, "A", 5, 15)
	b := seedDraftPack(t, "B", 5, 15)
	picks := picksOf(a, b, b)
	for i := range picks {
		picks[i].MarketPrice = nil
	}
	draft, err := db.CreateDraft(ctx, pool, 4, picks)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ApproveDraft(ctx, pool, draft.ID, seedAdmin(t).ID); err != nil {
		t.Fatal(err)
	}
	got, err := db.SetDraftCollected(ctx, pool, draft.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got {
		want := map[int]float64{a.ID: 10, b.ID: 20}[d.PackID]
		if d.Proceeds < want-0.001 || d.Proceeds > want+0.001 {
			t.Errorf("%s: want %v, got %v", d.SetName, want, d.Proceeds)
		}
	}
}

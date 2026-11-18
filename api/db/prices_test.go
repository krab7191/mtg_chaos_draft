package db_test

import (
	"context"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func day(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func TestPriceHistory_RecordBackfillAndRead(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "price_history", "price_history_backfill")
	ctx := context.Background()

	// Daily recording, including an update of the same day and ignored junk.
	if err := db.RecordPrices(ctx, pool, day("2026-11-01"), map[int]float64{1: 5, 2: 120, 3: 0}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordPrices(ctx, pool, day("2026-11-01"), map[int]float64{1: 5.5}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordPrices(ctx, pool, day("2026-11-02"), nil); err != nil {
		t.Fatal(err)
	}

	// Backfill keeps the recorded day and adds the rest.
	if err := db.StoreHistory(ctx, pool, 1, []db.PricePoint{
		{Day: day("2026-10-30"), Price: 6}, {Day: day("2026-10-31"), Price: 5.8},
		{Day: day("2026-11-01"), Price: 99}, {Day: day("2026-10-29"), Price: 0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreHistory(ctx, pool, 4, nil); err != nil { // nothing to store, still marked done
		t.Fatal(err)
	}

	done, err := db.BackfilledIDs(ctx, pool)
	if err != nil || !done[1] || !done[4] || done[2] {
		t.Fatalf("backfilled: %v %v", done, err)
	}

	hist, err := db.PriceHistory(ctx, pool, []int{1, 2, 3}, day("2026-10-31"))
	if err != nil {
		t.Fatal(err)
	}
	got := hist[1]
	if len(got) != 2 || got[0].Price != 5.8 || got[1].Price != 5.5 {
		t.Errorf("product 1: %+v", got)
	}
	if len(hist[2]) != 1 || hist[2][0].Price != 120 {
		t.Errorf("product 2: %+v", hist[2])
	}
	if _, ok := hist[3]; ok {
		t.Error("zero price should not be stored")
	}
}

func TestWatches(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "price_watches")
	ctx := context.Background()
	drop := 0.3

	if err := db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 2, DropAlert: &drop}); err != nil {
		t.Fatal(err)
	}
	// Updating replaces the threshold.
	if err := db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1, DropAlert: &drop}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 3, DropAlert: ptr(1.5)}); err == nil {
		t.Error("a drop of 150% should be rejected")
	}
	if err := db.DeleteWatch(ctx, pool, 2); err != nil {
		t.Fatal(err)
	}

	w, err := db.ListWatches(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 1 || w[1].DropAlert == nil || *w[1].DropAlert != 0.3 {
		t.Errorf("watches: %+v", w)
	}
}

func ptr(f float64) *float64 { return &f }

func TestWatchManyKeepsDropAlerts(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "price_watches")
	ctx := context.Background()
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1, DropAlert: ptr(0.4)})

	if err := db.WatchMany(ctx, pool, []int{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	w, _ := db.ListWatches(ctx, pool)
	if len(w) != 3 || w[1].DropAlert == nil || *w[1].DropAlert != 0.4 || w[2].DropAlert != nil {
		t.Errorf("after watching: %+v", w)
	}
	if err := db.UnwatchMany(ctx, pool, []int{1, 3}); err != nil {
		t.Fatal(err)
	}
	w, _ = db.ListWatches(ctx, pool)
	if len(w) != 1 || w[2].MTGStocksID != 2 {
		t.Errorf("after unwatching: %+v", w)
	}
}

func TestSetNotified(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "price_watches")
	ctx := context.Background()
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1})
	if w, _ := db.ListWatches(ctx, pool); len(w[1].Notified) != 0 {
		t.Errorf("new watch notified: %v", w[1].Notified)
	}
	if err := db.SetNotified(ctx, pool, 1, []string{"floor"}); err != nil {
		t.Fatal(err)
	}
	if w, _ := db.ListWatches(ctx, pool); len(w[1].Notified) != 1 || w[1].Notified[0] != "floor" {
		t.Errorf("notified: %v", w[1].Notified)
	}
	if err := db.SetNotified(ctx, pool, 1, nil); err != nil {
		t.Fatal(err)
	}
	if w, _ := db.ListWatches(ctx, pool); len(w[1].Notified) != 0 {
		t.Errorf("cleared: %v", w[1].Notified)
	}
}

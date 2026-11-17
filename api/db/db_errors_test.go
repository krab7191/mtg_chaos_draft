package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDB_DatabaseDown: every query function reports an error (rather than
// panicking or returning empty success) when Postgres is unavailable.
func TestDB_DatabaseDown(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	ctx := context.Background()
	price := 1.0
	qty := 1

	errOnly := func(_ any, err error) error { return err }
	errOnly3 := func(_, _ any, err error) error { return err }

	cases := map[string]func() error{
		"SaveCatalog": func() error {
			return db.SaveCatalog(ctx, pool, db.SavedCatalog{Body: []byte("[]"), FetchedAt: time.Now(), Source: "live"})
		},
		"LoadCatalog":       func() error { return errOnly(db.LoadCatalog(ctx, pool)) },
		"ListCollection":    func() error { return errOnly(db.ListCollection(ctx, pool)) },
		"AddPack":           func() error { return errOnly(db.AddPack(ctx, pool, 1, "P", "S", "", "Play Booster", 1, 1, &price, 14)) },
		"UpdatePack":        func() error { return errOnly(db.UpdatePack(ctx, pool, 1, &qty, nil, nil, nil, nil, nil)) },
		"DeletePack":        func() error { return db.DeletePack(ctx, pool, 1) },
		"BulkUpdatePrices":  func() error { return db.BulkUpdatePrices(ctx, pool, map[int]float64{1: 1}) },
		"GetPacksByIDs":     func() error { return errOnly(db.GetPacksByIDs(ctx, pool, []int{1})) },
		"RecordSale":        func() error { return errOnly(db.RecordSale(ctx, pool, 1, 1, 1, 0, "2026-01-01")) },
		"UpdateSale":        func() error { return errOnly(db.UpdateSale(ctx, pool, 1, 1, 1, 0, "2026-01-01")) },
		"DeleteSale":        func() error { return db.DeleteSale(ctx, pool, 1) },
		"SetDraftCollected": func() error { return errOnly(db.SetDraftCollected(ctx, pool, 1, 10)) },
		"ListDisposals":     func() error { return errOnly(db.ListDisposals(ctx, pool)) },
		"DeleteDraft":       func() error { return db.DeleteDraft(ctx, pool, 1) },
		"CreateDraft":       func() error { return errOnly(db.CreateDraft(ctx, pool, 2, []db.DraftPick{{SetName: "S"}})) },
		"ListDrafts":        func() error { return errOnly(db.ListDrafts(ctx, pool)) },
		"ApproveDraft":      func() error { return db.ApproveDraft(ctx, pool, 1, 1) },
		"AddPurchase":       func() error { return errOnly(db.AddPurchase(ctx, pool, 1, 1, 1, 0, "2026-01-01")) },
		"AddPackWithPurchase": func() error {
			return errOnly3(db.AddPackWithPurchase(ctx, pool, 1, "P", "S", "", "Play Booster", 1, 1, 1, &price, 14, 1, 0, "2026-01-01"))
		},
		"LogPurchase":           func() error { return errOnly(db.LogPurchase(ctx, pool, 1, 1, 1, 0, "2026-01-01", true)) },
		"ListPurchases":         func() error { return errOnly(db.ListPurchases(ctx, pool)) },
		"UpdatePurchase":        func() error { return errOnly(db.UpdatePurchase(ctx, pool, 1, 1, 1, 0, "2026-01-01")) },
		"DeletePurchase":        func() error { return db.DeletePurchase(ctx, pool, 1) },
		"CreateSession":         func() error { return db.CreateSession(ctx, pool, "s", 1, time.Now()) },
		"GetSession":            func() error { return errOnly(db.GetSession(ctx, pool, "s")) },
		"DeleteSession":         func() error { return db.DeleteSession(ctx, pool, "s") },
		"DeleteExpiredSessions": func() error { return db.DeleteExpiredSessions(ctx, pool) },
		"GetWeightSettings":     func() error { return errOnly(db.GetWeightSettings(ctx, pool)) },
		"UpdateWeightSettings":  func() error { return errOnly(db.UpdateWeightSettings(ctx, pool, &db.WeightSettings{})) },
		"GetOrCreateUser":       func() error { return errOnly(db.GetOrCreateUser(ctx, pool, "g", "e@x", "N", "")) },
		"GetUserByID":           func() error { return errOnly(db.GetUserByID(ctx, pool, 1)) },
		"SetUserRole":           func() error { return db.SetUserRole(ctx, pool, 1, "admin") },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("want error from a closed pool, got nil")
			}
		})
	}
}

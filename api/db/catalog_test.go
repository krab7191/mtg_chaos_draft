package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestCatalogSaveLoad(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "mtgstocks_catalog")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE mtgstocks_catalog`); err != nil {
		t.Fatal(err)
	}

	if _, err := db.LoadCatalog(ctx, pool); !errors.Is(err, db.ErrNoCatalog) {
		t.Fatalf("empty: want ErrNoCatalog, got %v", err)
	}

	first := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if err := db.SaveCatalog(ctx, pool, db.SavedCatalog{Body: []byte(`[1]`), FetchedAt: first, Source: "live"}); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}
	second := first.Add(time.Hour)
	if err := db.SaveCatalog(ctx, pool, db.SavedCatalog{Body: []byte(`[2]`), FetchedAt: second, Source: "upload"}); err != nil {
		t.Fatalf("SaveCatalog overwrite: %v", err)
	}

	c, err := db.LoadCatalog(ctx, pool)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if string(c.Body) != `[2]` || !c.FetchedAt.Equal(second) || c.Source != "upload" {
		t.Errorf("got %s %v %s", c.Body, c.FetchedAt, c.Source)
	}
}

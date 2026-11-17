package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoCatalog means no MTGStocks catalog has been saved yet.
var ErrNoCatalog = errors.New("no saved catalog")

// SavedCatalog is the stored raw MTGStocks /sealed response.
type SavedCatalog struct {
	Body      []byte
	FetchedAt time.Time
	// Source is "live" (fetched by the server) or "upload" (an admin snapshot).
	Source string
}

// SaveCatalog replaces the stored catalog.
func SaveCatalog(ctx context.Context, pool *pgxpool.Pool, c SavedCatalog) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO mtgstocks_catalog (id, body, fetched_at, source) VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET body = EXCLUDED.body, fetched_at = EXCLUDED.fetched_at, source = EXCLUDED.source
	`, c.Body, c.FetchedAt, c.Source)
	return err
}

// LoadCatalog returns the stored catalog, or ErrNoCatalog.
func LoadCatalog(ctx context.Context, pool *pgxpool.Pool) (*SavedCatalog, error) {
	c := &SavedCatalog{}
	err := pool.QueryRow(ctx, `SELECT body, fetched_at, source FROM mtgstocks_catalog WHERE id = 1`).
		Scan(&c.Body, &c.FetchedAt, &c.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCatalog
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

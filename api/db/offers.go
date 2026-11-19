package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Offer is one store listing of a single booster pack.
type Offer struct {
	Store       string    `json:"store"`
	URL         string    `json:"url"`
	MTGStocksID int       `json:"mtgstocksId"`
	Title       string    `json:"title"`
	Price       float64   `json:"price"`
	Available   bool      `json:"available"`
	SeenAt      time.Time `json:"seenAt"`
}

// ReplaceStoreOffers swaps a store's listings for a fresh complete set.
func ReplaceStoreOffers(ctx context.Context, pool *pgxpool.Pool, store string, offers []Offer) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM store_offers WHERE store = $1`, store); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, o := range offers {
		batch.Queue(`
			INSERT INTO store_offers (store, url, mtgstocks_id, title, price, available, seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (store, url) DO NOTHING`, store, o.URL, o.MTGStocksID, o.Title, o.Price, o.Available)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListStoreOffers returns every stored listing, cheapest first.
func ListStoreOffers(ctx context.Context, pool *pgxpool.Pool) ([]Offer, error) {
	rows, err := pool.Query(ctx, `
		SELECT store, url, mtgstocks_id, title, price::float8, available, seen_at
		FROM store_offers ORDER BY price, store, url`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.Store, &o.URL, &o.MTGStocksID, &o.Title, &o.Price, &o.Available, &o.SeenAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SetProductLink stores a product's TCGplayer page ("" when there is none).
func SetProductLink(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int, tcgURL string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO product_links (mtgstocks_id, tcg_url) VALUES ($1, $2)
		ON CONFLICT (mtgstocks_id) DO UPDATE SET tcg_url = EXCLUDED.tcg_url, fetched_at = NOW()`, mtgstocksID, tcgURL)
	return err
}

// ProductLinks returns every fetched TCGplayer page by product ("" = none).
func ProductLinks(ctx context.Context, pool *pgxpool.Pool) (map[int]string, error) {
	rows, err := pool.Query(ctx, `SELECT mtgstocks_id, tcg_url FROM product_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var id int
		var u string
		if err := rows.Scan(&id, &u); err != nil {
			return nil, err
		}
		out[id] = u
	}
	return out, rows.Err()
}

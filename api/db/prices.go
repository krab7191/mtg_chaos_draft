package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PricePoint is one day's market price for a product.
type PricePoint struct {
	Day   time.Time `json:"day"`
	Price float64   `json:"price"`
}

// RecordPrices stores one day's price for each product, replacing any price
// already stored for that day.
func RecordPrices(ctx context.Context, pool *pgxpool.Pool, day time.Time, prices map[int]float64) error {
	if len(prices) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for id, price := range prices {
		if price <= 0 {
			continue
		}
		batch.Queue(`
			INSERT INTO price_history (mtgstocks_id, day, price) VALUES ($1, $2, $3)
			ON CONFLICT (mtgstocks_id, day) DO UPDATE SET price = EXCLUDED.price`, id, day, price)
	}
	return pool.SendBatch(ctx, batch).Close()
}

// StoreHistory saves a product's backfilled history and marks it done, in one
// transaction. Days already recorded keep their price.
func StoreHistory(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int, points []PricePoint) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(points) > 0 {
		rows := make([][]any, 0, len(points))
		for _, p := range points {
			if p.Price > 0 {
				rows = append(rows, []any{mtgstocksID, p.Day, p.Price})
			}
		}
		// COPY into a temp table, then insert the days not yet recorded.
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE backfill_tmp (LIKE price_history) ON COMMIT DROP`); err != nil {
			return err
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"backfill_tmp"}, []string{"mtgstocks_id", "day", "price"}, pgx.CopyFromRows(rows)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO price_history SELECT * FROM backfill_tmp ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO price_history_backfill (mtgstocks_id) VALUES ($1)
		ON CONFLICT (mtgstocks_id) DO UPDATE SET done_at = NOW()`, mtgstocksID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BackfilledIDs returns the products whose history has been fetched.
func BackfilledIDs(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT mtgstocks_id FROM price_history_backfill`)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return nil, err
	}
	done := make(map[int]bool, len(ids))
	for _, id := range ids {
		done[id] = true
	}
	return done, nil
}

// PriceHistory returns each product's daily prices since `since`, oldest first.
func PriceHistory(ctx context.Context, pool *pgxpool.Pool, ids []int, since time.Time) (map[int][]PricePoint, error) {
	rows, err := pool.Query(ctx, `
		SELECT mtgstocks_id, day, price::float8 FROM price_history
		WHERE mtgstocks_id = ANY($1) AND day >= $2
		ORDER BY mtgstocks_id, day`, ids, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]PricePoint, len(ids))
	for rows.Next() {
		var id int
		var p PricePoint
		if err := rows.Scan(&id, &p.Day, &p.Price); err != nil {
			return nil, err
		}
		out[id] = append(out[id], p)
	}
	return out, rows.Err()
}

// Watch is a product watched for alerts; DropAlert is an optional fraction
// below the release price that should raise an alert.
type Watch struct {
	MTGStocksID int      `json:"mtgstocksId"`
	DropAlert   *float64 `json:"dropAlert"`
	// Notified lists the alert reasons already sent to the phone.
	Notified []string `json:"-"`
}

// SetWatch starts or updates watching a product.
func SetWatch(ctx context.Context, pool *pgxpool.Pool, w Watch) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO price_watches (mtgstocks_id, drop_alert) VALUES ($1, $2)
		ON CONFLICT (mtgstocks_id) DO UPDATE SET drop_alert = EXCLUDED.drop_alert`, w.MTGStocksID, w.DropAlert)
	return err
}

// DeleteWatch stops watching a product.
func DeleteWatch(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int) error {
	_, err := pool.Exec(ctx, `DELETE FROM price_watches WHERE mtgstocks_id = $1`, mtgstocksID)
	return err
}

// ListWatches returns every watched product by MTGStocks id.
func ListWatches(ctx context.Context, pool *pgxpool.Pool) (map[int]Watch, error) {
	rows, err := pool.Query(ctx, `SELECT mtgstocks_id, drop_alert::float8, notified FROM price_watches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]Watch{}
	for rows.Next() {
		var w Watch
		if err := rows.Scan(&w.MTGStocksID, &w.DropAlert, &w.Notified); err != nil {
			return nil, err
		}
		out[w.MTGStocksID] = w
	}
	return out, rows.Err()
}

// WatchMany starts watching several products, keeping the drop alert of any
// already watched.
func WatchMany(ctx context.Context, pool *pgxpool.Pool, ids []int) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO price_watches (mtgstocks_id) SELECT unnest($1::int[])
		ON CONFLICT (mtgstocks_id) DO NOTHING`, ids)
	return err
}

// UnwatchMany stops watching several products.
func UnwatchMany(ctx context.Context, pool *pgxpool.Pool, ids []int) error {
	_, err := pool.Exec(ctx, `DELETE FROM price_watches WHERE mtgstocks_id = ANY($1)`, ids)
	return err
}

// SetNotified records which alert reasons have been sent for a watch.
func SetNotified(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int, reasons []string) error {
	if reasons == nil {
		reasons = []string{}
	}
	_, err := pool.Exec(ctx, `UPDATE price_watches SET notified = $2 WHERE mtgstocks_id = $1`, mtgstocksID, reasons)
	return err
}

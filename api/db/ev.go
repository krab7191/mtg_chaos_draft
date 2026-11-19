package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoredBoosters is a set's MTGJSON pull-rate data and when it was fetched.
type StoredBoosters struct {
	Boosters  json.RawMessage
	FetchedAt time.Time
}

// SaveBoosters stores a set's pull-rate data (by lowercase set code).
func SaveBoosters(ctx context.Context, pool *pgxpool.Pool, setCode string, boosters json.RawMessage) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO mtgjson_boosters (set_code, boosters) VALUES ($1, $2)
		ON CONFLICT (set_code) DO UPDATE SET boosters = EXCLUDED.boosters, fetched_at = NOW()`, setCode, boosters)
	return err
}

// LoadBoosters returns every stored set's pull-rate data.
func LoadBoosters(ctx context.Context, pool *pgxpool.Pool) (map[string]StoredBoosters, error) {
	rows, err := pool.Query(ctx, `SELECT set_code, boosters, fetched_at FROM mtgjson_boosters`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]StoredBoosters{}
	for rows.Next() {
		var code string
		var b StoredBoosters
		if err := rows.Scan(&code, &b.Boosters, &b.FetchedAt); err != nil {
			return nil, err
		}
		out[code] = b
	}
	return out, rows.Err()
}

// RecordEV stores pack EVs by day, replacing any already stored.
func RecordEV(ctx context.Context, pool *pgxpool.Pool, byDay map[time.Time]map[int]float64) error {
	batch := &pgx.Batch{}
	for day, evs := range byDay {
		for id, ev := range evs {
			batch.Queue(`
				INSERT INTO pack_ev (mtgstocks_id, day, ev) VALUES ($1, $2, $3)
				ON CONFLICT (mtgstocks_id, day) DO UPDATE SET ev = EXCLUDED.ev`, id, day, ev)
		}
	}
	if batch.Len() == 0 {
		return nil
	}
	return pool.SendBatch(ctx, batch).Close()
}

// EVDays is how many distinct days of EV are stored.
func EVDays(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT day) FROM pack_ev`).Scan(&n)
	return n, err
}

// PackEV is a pack's latest EV and the EV about two weeks before it.
type PackEV struct {
	Value  float64
	Day    time.Time
	Before *float64 // the EV ago days earlier (or the closest day before), if any
}

// LatestEV returns each product's latest EV and its EV `ago` days before.
func LatestEV(ctx context.Context, pool *pgxpool.Pool, ids []int, ago int) (map[int]PackEV, error) {
	rows, err := pool.Query(ctx, `
		SELECT l.mtgstocks_id, l.ev::float8, l.day,
		       (SELECT p.ev::float8 FROM pack_ev p
		        WHERE p.mtgstocks_id = l.mtgstocks_id AND p.day <= l.day - $2::int
		        ORDER BY p.day DESC LIMIT 1)
		FROM (SELECT DISTINCT ON (mtgstocks_id) mtgstocks_id, ev, day FROM pack_ev
		      WHERE mtgstocks_id = ANY($1) ORDER BY mtgstocks_id, day DESC) l`, ids, ago)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]PackEV{}
	for rows.Next() {
		var id int
		var e PackEV
		if err := rows.Scan(&id, &e.Value, &e.Day, &e.Before); err != nil {
			return nil, err
		}
		out[id] = e
	}
	return out, rows.Err()
}

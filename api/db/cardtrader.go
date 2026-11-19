package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CTBlueprint is a CardTrader booster product matched to an MTGStocks pack.
type CTBlueprint struct {
	BlueprintID int
	ExpansionID int
	MTGStocksID int
	Name        string
}

// CardTraderScanned returns the CardTrader expansions already scanned.
func CardTraderScanned(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT expansion_id FROM cardtrader_expansions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SaveCardTraderExpansion records an expansion as scanned with the boosters
// found in it, in one transaction.
func SaveCardTraderExpansion(ctx context.Context, pool *pgxpool.Pool, expansionID int, found []CTBlueprint) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, b := range found {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cardtrader_blueprints (blueprint_id, expansion_id, mtgstocks_id, name) VALUES ($1, $2, $3, $4)
			ON CONFLICT (blueprint_id) DO UPDATE SET mtgstocks_id = EXCLUDED.mtgstocks_id, name = EXCLUDED.name`,
			b.BlueprintID, expansionID, b.MTGStocksID, b.Name); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO cardtrader_expansions (expansion_id) VALUES ($1)
		ON CONFLICT (expansion_id) DO UPDATE SET scanned_at = NOW()`, expansionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CardTraderBlueprints returns every matched booster.
func CardTraderBlueprints(ctx context.Context, pool *pgxpool.Pool) ([]CTBlueprint, error) {
	rows, err := pool.Query(ctx, `SELECT blueprint_id, expansion_id, mtgstocks_id, name FROM cardtrader_blueprints ORDER BY blueprint_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CTBlueprint
	for rows.Next() {
		var b CTBlueprint
		if err := rows.Scan(&b.BlueprintID, &b.ExpansionID, &b.MTGStocksID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

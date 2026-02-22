package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WeightSettings struct {
	PriceSensitivity     float64         `json:"priceSensitivity"`
	ScaricitySensitivity float64         `json:"scarcitySensitivity"`
	PriceCap             float64         `json:"priceCap"`
	PriceFloor           float64         `json:"priceFloor"`
	QuantityCap          int             `json:"quantityCap"`
	PackWeights          map[int]float64 `json:"packWeights"`
}

func GetWeightSettings(ctx context.Context, pool *pgxpool.Pool) (*WeightSettings, error) {
	s := &WeightSettings{}
	err := pool.QueryRow(ctx, `
		SELECT price_sensitivity, scarcity_sensitivity, price_cap, price_floor, quantity_cap
		FROM weight_settings WHERE id = 1
	`).Scan(&s.PriceSensitivity, &s.ScaricitySensitivity, &s.PriceCap, &s.PriceFloor, &s.QuantityCap)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `SELECT pack_id, multiplier FROM pack_weight_overrides`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	s.PackWeights = make(map[int]float64)
	for rows.Next() {
		var id int
		var mult float64
		if err := rows.Scan(&id, &mult); err != nil {
			return nil, err
		}
		s.PackWeights[id] = mult
	}
	return s, rows.Err()
}

func UpdateWeightSettings(ctx context.Context, pool *pgxpool.Pool, s *WeightSettings) (*WeightSettings, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE weight_settings
		SET price_sensitivity = $1, scarcity_sensitivity = $2,
		    price_cap = $3, price_floor = $4, quantity_cap = $5
		WHERE id = 1
	`, s.PriceSensitivity, s.ScaricitySensitivity, s.PriceCap, s.PriceFloor, s.QuantityCap)
	if err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx, `DELETE FROM pack_weight_overrides`); err != nil {
		return nil, err
	}
	for packID, mult := range s.PackWeights {
		if _, err = tx.Exec(ctx,
			`INSERT INTO pack_weight_overrides (pack_id, multiplier) VALUES ($1, $2)`,
			packID, mult,
		); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return GetWeightSettings(ctx, pool)
}

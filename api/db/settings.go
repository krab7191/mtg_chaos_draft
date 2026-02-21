package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WeightSettings struct {
	PriceSensitivity    float64 `json:"priceSensitivity"`
	ScaricitySensitivity float64 `json:"scarcitySensitivity"`
}

func GetWeightSettings(ctx context.Context, pool *pgxpool.Pool) (*WeightSettings, error) {
	s := &WeightSettings{}
	err := pool.QueryRow(ctx, `
		SELECT price_sensitivity, scarcity_sensitivity FROM weight_settings WHERE id = 1
	`).Scan(&s.PriceSensitivity, &s.ScaricitySensitivity)
	return s, err
}

func UpdateWeightSettings(ctx context.Context, pool *pgxpool.Pool, priceSensitivity, scarcitySensitivity float64) (*WeightSettings, error) {
	s := &WeightSettings{}
	err := pool.QueryRow(ctx, `
		UPDATE weight_settings
		SET price_sensitivity = $1, scarcity_sensitivity = $2
		WHERE id = 1
		RETURNING price_sensitivity, scarcity_sensitivity
	`, priceSensitivity, scarcitySensitivity).Scan(&s.PriceSensitivity, &s.ScaricitySensitivity)
	return s, err
}

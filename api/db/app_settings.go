package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetSetting reads an app setting into out; ok=false when it isn't set.
func GetSetting(ctx context.Context, pool *pgxpool.Pool, key string, out any) (bool, error) {
	var raw json.RawMessage
	err := pool.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, out)
}

// SetSetting stores an app setting.
func SetSetting(ctx context.Context, pool *pgxpool.Pool, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO app_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, raw)
	return err
}

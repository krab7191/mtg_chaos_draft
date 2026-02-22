package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Retry ping for up to 30 seconds (useful when postgres is still starting)
	for i := range 10 {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		if i == 9 {
			return nil, fmt.Errorf("ping db after retries: %w", err)
		}
		fmt.Printf("waiting for database... (%d/10)\n", i+1)
		time.Sleep(3 * time.Second)
	}

	if err := runMigrations(ctx, pool); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return pool, nil
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var count int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE filename = $1`, entry.Name(),
		).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}
		if count > 0 {
			continue
		}
		sql, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, entry.Name(),
		); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
		fmt.Printf("applied migration %s\n", entry.Name())
	}
	return nil
}

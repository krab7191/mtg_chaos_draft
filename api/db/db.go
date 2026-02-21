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
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sql, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

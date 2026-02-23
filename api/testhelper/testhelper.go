// Package testhelper connects to a real Postgres instance for integration tests.
// DATABASE_URL must point to a test database. Run `make db-test` to create it.
// Tests are skipped automatically when the database is unreachable.
package testhelper

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mtg-chaos-draft/db"
)

const defaultURL = "postgres://mtg:mtg@localhost:5432/mtg_chaos_draft_test?sslmode=disable"

var (
	once     sync.Once
	pool     *pgxpool.Pool
	setupErr error
)

// Pool returns a *pgxpool.Pool connected to the test database with all
// migrations applied. The connection is created once per test binary run.
// Tests are skipped if the database is unreachable.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	once.Do(func() {
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			url = defaultURL
		}
		pool, setupErr = db.New(context.Background(), url)
	})
	if setupErr != nil {
		t.Skipf("database unavailable (run `make db`): %v", setupErr)
	}
	return pool
}

// Truncate clears the given tables (restarting identity sequences) after the test.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, tbl := range tables {
			if _, err := pool.Exec(context.Background(),
				"TRUNCATE "+tbl+" RESTART IDENTITY CASCADE"); err != nil {
				t.Errorf("truncate %s: %v", tbl, err)
			}
		}
	})
}

// Package testhelper manages the test Postgres instance for integration tests.
// Setup() ensures postgres is running (starting it via docker compose if needed)
// and that the test database exists. Tests call Pool() to get a connection.
package testhelper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mtg-chaos-draft/db"
)

const defaultURL = "postgres://mtg:mtg@localhost:5432/mtg_chaos_draft_test?sslmode=disable"

var (
	once     sync.Once
	pool     *pgxpool.Pool
	setupErr error
)

// projectRoot walks up from the working directory to find docker-compose.yml.
func projectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// pgReachable returns true if postgres is accepting connections.
// It connects to the system "postgres" database so it works even before the
// test database has been created.
func pgReachable(url string) bool {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return false
	}
	cfg.Database = "postgres"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return false
	}
	conn.Close(ctx)
	return true
}

// ensureTestDB connects to the system "postgres" database and creates the test
// database if it does not already exist.
func ensureTestDB(testURL string) {
	cfg, err := pgx.ParseConfig(testURL)
	if err != nil {
		return
	}
	cfg.Database = "postgres"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	_, _ = conn.Exec(ctx, "CREATE DATABASE mtg_chaos_draft_test")
}

// Setup ensures postgres is running and the test database exists.
// If postgres is already listening (e.g. a CI service container or local dev
// instance) it is used as-is. Otherwise postgres is started via docker compose.
// Call this from TestMain before m.Run().
func Setup() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = defaultURL
	}

	if !pgReachable(url) {
		root := projectRoot()
		cmd := exec.Command("docker", "compose", "up", "postgres", "-d")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "docker compose up: %v\n%s\n", err, out)
		}
		for !pgReachable(url) {
			// spin until postgres accepts connections
		}
	}

	ensureTestDB(url)
}

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
		t.Skipf("database unavailable: %v", setupErr)
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

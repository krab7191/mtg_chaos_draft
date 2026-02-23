// Package testhelper manages the test Postgres instance for integration tests.
// Calling Setup() starts postgres via docker compose and creates the test DB.
// Tests that need the DB call Pool(); they are skipped if the DB is unreachable.
package testhelper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// projectRoot walks up from the current working directory to find the repo
// root (identified by the presence of docker-compose.yml).
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

// Setup starts postgres via docker compose and ensures the test database
// exists. It is idempotent — safe to call when postgres is already running.
// Call this from TestMain before m.Run().
func Setup() {
	root := projectRoot()

	run := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "testhelper.Setup %v: %v\n%s\n", args, err, out)
		}
		return err
	}

	// Start postgres (no-op if already running)
	_ = run("docker", "compose", "up", "postgres", "-d")

	// Wait until postgres is ready to accept connections
	for {
		cmd := exec.Command("docker", "compose", "exec", "postgres", "pg_isready", "-U", "mtg", "-q")
		cmd.Dir = root
		if cmd.Run() == nil {
			break
		}
	}

	// Create the test database (ignore error — it may already exist)
	_ = run("docker", "compose", "exec", "postgres",
		"psql", "-U", "mtg", "-d", "postgres", "-c", "CREATE DATABASE mtg_chaos_draft_test;")
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

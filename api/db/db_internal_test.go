package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func fastPingRetries(t *testing.T) {
	t.Helper()
	origAttempts, origDelay := pingAttempts, pingRetryDelay
	pingAttempts, pingRetryDelay = 2, time.Millisecond
	t.Cleanup(func() { pingAttempts, pingRetryDelay = origAttempts, origDelay })
}

const unreachableDB = "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"

func TestNew_ExistingDatabase(t *testing.T) {
	pool, err := New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	pool.Close()
}

// TestNew_FreshDatabaseAppliesEveryMigration runs all migrations from scratch
// on a throwaway database.
func TestNew_FreshDatabaseAppliesEveryMigration(t *testing.T) {
	base := os.Getenv("DATABASE_URL")
	admin, err := pgxpool.New(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("mtg_fresh_%d", time.Now().UnixNano())
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Skipf("cannot create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })

	u, _ := url.Parse(base)
	u.Path = "/" + name
	pool, err := New(context.Background(), u.String())
	if err != nil {
		t.Fatalf("New on fresh db: %v", err)
	}
	defer pool.Close()

	entries, _ := migrations.ReadDir("migrations")
	var applied int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(entries) {
		t.Errorf("want %d migrations applied, got %d", len(entries), applied)
	}
}

func TestNew_InvalidURL(t *testing.T) {
	if _, err := New(context.Background(), "::not a url"); err == nil || !strings.Contains(err.Error(), "create pool") {
		t.Errorf("want create pool error, got %v", err)
	}
}

func TestNew_GivesUpAfterRetries(t *testing.T) {
	fastPingRetries(t)
	if _, err := New(context.Background(), unreachableDB); err == nil || !strings.Contains(err.Error(), "after retries") {
		t.Errorf("want retries error, got %v", err)
	}
}

func TestNew_StopsWhenCancelled(t *testing.T) {
	origAttempts, origDelay := pingAttempts, pingRetryDelay
	pingAttempts, pingRetryDelay = 10, time.Hour
	t.Cleanup(func() { pingAttempts, pingRetryDelay = origAttempts, origDelay })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := New(ctx, unreachableDB); err == nil {
		t.Fatal("want error")
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("New ignored cancellation")
	}
}

func TestRunMigrations_DatabaseDown(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if err := runMigrations(context.Background(), pool); err == nil || !strings.Contains(err.Error(), "schema_migrations") {
		t.Errorf("want schema_migrations error, got %v", err)
	}
}

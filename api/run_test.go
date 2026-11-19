package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testEnv is a complete environment for run(), on a free port.
func testEnv(t *testing.T) map[string]string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return map[string]string{
		"DATABASE_URL":         os.Getenv("DATABASE_URL"),
		"GOOGLE_REDIRECT_URL":  "https://example.test/api/auth/callback",
		"GOOGLE_CLIENT_ID":     "id",
		"GOOGLE_CLIENT_SECRET": "secret",
		"ADMIN_EMAIL":          "admin@example.test",
		"VIEWER_EMAILS":        "viewer@example.test",
		"PORT":                 fmt.Sprint(port),
	}
}

func getenvFrom(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

// stubMaintenance swaps the price refresh for a counter.
func stubMaintenance(t *testing.T, interval time.Duration) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	origRefresh, origInterval, origBackfill, origMarkets := refreshPrices, maintenanceInterval, backfillPrices, refreshMarkets
	refreshPrices = func(context.Context, *pgxpool.Pool) { calls.Add(1) }
	backfillPrices = func(context.Context, *pgxpool.Pool) (int, error) { return 0, nil }
	refreshMarkets = func(context.Context, *pgxpool.Pool) {}
	maintenanceInterval = interval
	t.Cleanup(func() {
		refreshPrices, maintenanceInterval, backfillPrices, refreshMarkets = origRefresh, origInterval, origBackfill, origMarkets
	})
	return &calls
}

func TestRun_ServesAndShutsDownGracefully(t *testing.T) {
	calls := stubMaintenance(t, 10*time.Millisecond)
	env := testEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, getenvFrom(env), "/nonexistent") }()

	url := "http://127.0.0.1:" + env["PORT"] + "/api/health"
	var healthy bool
	for i := 0; i < 100 && !healthy; i++ {
		if res, err := http.Get(url); err == nil {
			healthy = res.StatusCode == http.StatusOK
			_ = res.Body.Close()
		}
		if !healthy {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !healthy {
		t.Fatal("server never became healthy")
	}

	// Maintenance runs immediately and then on every tick.
	for i := 0; i < 100 && calls.Load() < 2; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() < 2 {
		t.Errorf("want repeated maintenance runs, got %d", calls.Load())
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func TestRun_DefaultsPortTo8080(t *testing.T) {
	stubMaintenance(t, time.Hour)
	env := testEnv(t)
	delete(env, "PORT")
	// Occupy 8080 if free so run fails fast with a listen error mentioning it;
	// either way run must try :8080.
	l, err := net.Listen("tcp", ":8080")
	if err == nil {
		defer l.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = run(ctx, getenvFrom(env), "/nonexistent")
	if err == nil || !strings.Contains(err.Error(), "8080") {
		t.Errorf("want listen error on :8080, got %v", err)
	}
}

func TestRun_MissingEnv(t *testing.T) {
	err := run(context.Background(), getenvFrom(map[string]string{"DATABASE_URL": "x"}), "/nonexistent")
	if err == nil || !strings.Contains(err.Error(), "GOOGLE_CLIENT_ID") || !strings.Contains(err.Error(), "ADMIN_EMAIL") {
		t.Errorf("want missing-env error naming the vars, got %v", err)
	}
}

func TestRun_BadDatabase(t *testing.T) {
	env := testEnv(t)
	env["DATABASE_URL"] = "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"
	// Startup gives up when its context ends instead of retrying for 30s.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := run(ctx, getenvFrom(env), "/nonexistent"); err == nil || !strings.Contains(err.Error(), "db init") {
		t.Errorf("want db init error, got %v", err)
	}
}

func TestRunMaintenance_LogsSessionCleanupErrors(t *testing.T) {
	calls := stubMaintenance(t, time.Hour)
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	pool.Close() // every query now fails
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runMaintenance(ctx, pool, time.Hour); close(done) }()
	for i := 0; i < 100 && calls.Load() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if calls.Load() != 1 {
		t.Errorf("want one refresh, got %d", calls.Load())
	}
}

func TestRunBackfill_RetriesUntilCancelled(t *testing.T) {
	var calls atomic.Int32
	orig := backfillPrices
	backfillPrices = func(context.Context, *pgxpool.Pool) (int, error) {
		if calls.Add(1) == 1 {
			return 0, fmt.Errorf("tunnel down") // logged, retried next tick
		}
		return 2, nil
	}
	t.Cleanup(func() { backfillPrices = orig })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runBackfill(ctx, nil, 5*time.Millisecond); close(done) }()
	for i := 0; i < 200 && calls.Load() < 3; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if calls.Load() < 3 {
		t.Errorf("want repeated runs after a failure, got %d", calls.Load())
	}
}

func TestRunEvery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var n atomic.Int32
	done := make(chan struct{})
	go func() { runEvery(ctx, 5*time.Millisecond, func() { n.Add(1) }); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
	if n.Load() < 2 {
		t.Errorf("ran %d times", n.Load())
	}
}

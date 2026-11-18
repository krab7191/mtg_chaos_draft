package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

// fakeNtfy records the notifications it receives; status sets its reply.
type fakeNtfy struct {
	mu     sync.Mutex
	got    []http.Header
	bodies []string
	status int
}

func newFakeNtfy(t *testing.T) *fakeNtfy {
	t.Helper()
	f := &fakeNtfy{status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		f.got = append(f.got, r.Header.Clone())
		f.bodies = append(f.bodies, string(body))
		w.WriteHeader(f.status)
	}))
	t.Cleanup(srv.Close)
	ConfigureNotifications(" "+srv.URL+"/mtg-topic ", "https://cards.example/")
	t.Cleanup(func() { ConfigureNotifications("", "") })
	return f
}

func (f *fakeNtfy) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.got) }

// seedFloor: product 1 (released 2023-01-01) fell from $10 to price and has
// sat there since, up to today.
func seedFloor(t *testing.T, ctx context.Context, price float64) {
	t.Helper()
	pool := testhelper.Pool(t)
	start := day("2023-01-01")
	days := int(time.Now().UTC().Sub(start).Hours()/24) + 1
	prices := append(repeat(10, 20), repeat(price, days-20)...)
	if _, err := pool.Exec(ctx, `TRUNCATE price_history`); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreHistory(ctx, pool, 1, series("2023-01-01", prices...)); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyAlerts_SendsOncePerAlertAndAgainAfterItClears(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	drop := 0.3
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1, DropAlert: &drop})
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 2}) // watched, no history: no alert
	seedFloor(t, ctx, 6)

	run := func() int {
		var n int
		var err error
		withMockClient(200, fakeSetsJSON, func() { n, err = NotifyAlerts(ctx, pool) })
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if n := run(); n != 2 { // floor + drop
		t.Fatalf("first run: want 2 notifications, got %d", n)
	}
	h := ntfy.got[0]
	if !strings.HasPrefix(h.Get("Title"), "Possible floor: Test Set Draft Booster") || h.Get("Click") != "https://cards.example/admin/trends" {
		t.Errorf("headers: %v", h)
	}
	if !strings.Contains(ntfy.bodies[0], "$6.00") || !strings.Contains(ntfy.bodies[0], "-40% since release") {
		t.Errorf("floor body: %q", ntfy.bodies[0])
	}
	if !strings.HasPrefix(ntfy.got[1].Get("Title"), "Price drop:") || !strings.Contains(ntfy.bodies[1], "your alert: -30%") {
		t.Errorf("drop alert: %v %q", ntfy.got[1].Get("Title"), ntfy.bodies[1])
	}

	if n := run(); n != 0 {
		t.Errorf("same alerts again: want 0, got %d", n)
	}

	// The price recovers: alerts clear and are forgotten...
	seedFloor(t, ctx, 9.9)
	if n := run(); n != 0 {
		t.Errorf("recovered: want 0, got %d", n)
	}
	if w, _ := db.ListWatches(ctx, pool); len(w[1].Notified) != 0 {
		t.Errorf("cleared alerts should be forgotten: %v", w[1].Notified)
	}
	// ...so a later floor notifies again.
	seedFloor(t, ctx, 6)
	if n := run(); n != 2 {
		t.Errorf("new floor: want 2, got %d", n)
	}
}

func TestNotifyAlerts_RetriesFailedSends(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	ntfy.status = http.StatusInternalServerError
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1})
	seedFloor(t, ctx, 6)

	withMockClient(200, fakeSetsJSON, func() {
		if n, err := NotifyAlerts(ctx, pool); err != nil || n != 0 {
			t.Errorf("failed send: %d %v", n, err)
		}
		ntfy.status = http.StatusOK
		if n, _ := NotifyAlerts(ctx, pool); n != 1 {
			t.Errorf("retry: want 1, got %d", n)
		}
	})
}

func TestNotifyAlerts_OffOrNothingToDo(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	if n, err := NotifyAlerts(ctx, pool); n != 0 || err != nil {
		t.Errorf("off: %d %v", n, err)
	}
	newFakeNtfy(t)
	if n, err := NotifyAlerts(ctx, pool); n != 0 || err != nil {
		t.Errorf("no watches: %d %v", n, err)
	}
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1})
	withMockClient(403, "", func() {
		if _, err := NotifyAlerts(ctx, pool); err == nil {
			t.Error("no catalog should be an error")
		}
	})
	if _, err := NotifyAlerts(ctx, closedPool(t)); err == nil {
		t.Error("db down should be an error")
	}
}

func TestSendNotification_Errors(t *testing.T) {
	ConfigureNotifications("http://127.0.0.1:1/topic", "")
	t.Cleanup(func() { ConfigureNotifications("", "") })
	if err := sendNotification(context.Background(), "t", "m"); err == nil {
		t.Error("unreachable ntfy should fail")
	}
	ConfigureNotifications("::bad", "")
	if err := sendNotification(context.Background(), "t", "m"); err == nil {
		t.Error("bad URL should fail")
	}
}

func TestNotifyTestEndpoint(t *testing.T) {
	h := &TrendsHandler{now: time.Now}
	call := func() *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.NotifyTest(rr, httptest.NewRequest(http.MethodPost, "/api/trends/notify-test", nil))
		return rr
	}
	ConfigureNotifications("", "")
	if rr := call(); rr.Code != 400 || !strings.Contains(rr.Body.String(), "NTFY_URL") {
		t.Errorf("not set up: %d %s", rr.Code, rr.Body)
	}
	ntfy := newFakeNtfy(t)
	if rr := call(); rr.Code != 204 || ntfy.count() != 1 || !strings.Contains(ntfy.got[0].Get("Title"), "test alert") {
		t.Errorf("test alert: %d", rr.Code)
	}
	ntfy.status = 500
	if rr := call(); rr.Code != 502 {
		t.Errorf("ntfy down: %d", rr.Code)
	}
}

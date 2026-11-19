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

// seedFloor: product 1 (released 2023-01-01) fell from $10 to just above
// price, then settled at price for the last 30 days (a recent low), up to today.
func seedFloor(t *testing.T, ctx context.Context, price float64) {
	t.Helper()
	pool := testhelper.Pool(t)
	start := day("2023-01-01")
	days := int(time.Now().UTC().Sub(start).Hours()/24) + 1
	prices := append(append(repeat(10, 20), repeat(price+0.5, days-50)...), repeat(price, 30)...)
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

	// Floor + drop on the same day: one summary notification, not two.
	if n := run(); n != 1 {
		t.Fatalf("first run: want 1 notification, got %d", n)
	}
	h := ntfy.got[0]
	if h.Get("Title") != "2 new price alerts" || h.Get("Click") != "https://cards.example/admin/trends" {
		t.Errorf("headers: %v", h)
	}
	body := ntfy.bodies[0]
	if !strings.Contains(body, "◆ Test Set Draft Booster: possible floor at $6.00, -40% since release") ||
		!strings.Contains(body, "↓ Test Set Draft Booster: $6.00, -40% since release (alert -30%)") ||
		strings.Count(body, "\n") != 1 {
		t.Errorf("summary body: %q", body)
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
	if n := run(); n != 1 || ntfy.count() != 2 {
		t.Errorf("new floor: want 1 more notification, got %d (total %d)", n, ntfy.count())
	}
}

func TestNotifyAlerts_SingleAlertGetsItsOwnMessage(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	_ = db.SetWatch(ctx, pool, db.Watch{MTGStocksID: 1})
	seedFloor(t, ctx, 6)
	withMockClient(200, fakeSetsJSON, func() {
		if n, err := NotifyAlerts(ctx, pool); n != 1 || err != nil {
			t.Fatalf("%d %v", n, err)
		}
	})
	if !strings.HasPrefix(ntfy.got[0].Get("Title"), "Possible floor: Test Set Draft Booster") ||
		!strings.Contains(ntfy.bodies[0], "Steady near its low ($6.00)") {
		t.Errorf("single: %v %q", ntfy.got[0].Get("Title"), ntfy.bodies[0])
	}
}

func TestWatching_MarksExistingAlertsSeen(t *testing.T) {
	ctx := trendsDB(t)
	pool := testhelper.Pool(t)
	ntfy := newFakeNtfy(t)
	seedFloor(t, ctx, 6) // product 1 is already at a floor
	h := newTestTrendsHandler(t)

	withMockClient(200, fakeSetsJSON, func() {
		// Mass-watch: the page shows the floor right away...
		rr := httptest.NewRecorder()
		h.SetWatchMany(rr, httptest.NewRequest(http.MethodPut, "/api/trends/watch", strings.NewReader(`{"ids":[1,2],"watch":true}`)))
		if rr.Code != 204 {
			t.Fatalf("watch many: %d", rr.Code)
		}
		// ...so the daily check doesn't buzz the phone about it.
		if n, _ := NotifyAlerts(ctx, pool); n != 0 || ntfy.count() != 0 {
			t.Errorf("already-seen alerts were sent: %d", ntfy.count())
		}

		// Adding a drop alert that's already exceeded: also seen, not sent.
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"watch":true,"dropAlert":0.3}`))
		req.SetPathValue("mtgstocksId", "1")
		h.SetWatch(httptest.NewRecorder(), req)
		if n, _ := NotifyAlerts(ctx, pool); n != 0 {
			t.Errorf("drop alert set on screen was sent: %d", n)
		}
	})
	if w, _ := db.ListWatches(ctx, pool); strings.Join(w[1].Notified, ",") != "floor,drop" {
		t.Errorf("seen: %v", w[1].Notified)
	}

	// Without a catalog, marking seen is skipped (logged), not an error.
	withMockClient(403, "", func() {
		resetProductsCache()
		markAlertsSeen(ctx, pool, []int{1})
	})
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

func TestAlertTextForNewLow(t *testing.T) {
	p := SearchResult{SetName: "Bloomburrow", ProductType: "Play Booster Box", PacksPerBox: 36}
	tr := &Trend{Current: 108, PrevLow: 112.5, DropFromRelease: 0.4}
	title, body := alertMessage(p, tr, &db.Watch{}, AlertNewLow)
	if title != "New low: Bloomburrow Play Booster Box" || !strings.Contains(body, "previous low ($112.50)") {
		t.Errorf("%q %q", title, body)
	}
	line := alertLine(watchedAlert{product: p, trend: tr}, AlertNewLow)
	if line != "▼ Bloomburrow Play Booster Box: new low at $108.00, below its previous low of $112.50" {
		t.Errorf("line: %q", line)
	}
}

package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── mock transports ────────────────────────────────────────────────────────────

type stubTransport struct {
	statusCode int
	body       string
}

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.statusCode,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

type errTransport struct{}

func (errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("simulated network error")
}

func withMockClient(statusCode int, body string, f func()) {
	orig := mtgstocksClient
	mtgstocksClient = &http.Client{Transport: &stubTransport{statusCode: statusCode, body: body}}
	resetMTGStocksBackoff()
	defer func() { mtgstocksClient = orig; resetMTGStocksBackoff() }()
	f()
}

func withErrClient(f func()) {
	orig := mtgstocksClient
	mtgstocksClient = &http.Client{Transport: errTransport{}}
	resetMTGStocksBackoff()
	defer func() { mtgstocksClient = orig; resetMTGStocksBackoff() }()
	f()
}

// ── proxyMTGStocks ─────────────────────────────────────────────────────────────

func TestProxyMTGStocks_Success(t *testing.T) {
	withMockClient(200, `{"ok":true}`, func() {
		body, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != 200 {
			t.Errorf("want 200, got %d", status)
		}
		if string(body) != `{"ok":true}` {
			t.Errorf("unexpected body: %s", body)
		}
	})
}

func TestProxyMTGStocks_Non200(t *testing.T) {
	withMockClient(404, `not found`, func() {
		_, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/9999")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != 404 {
			t.Errorf("want 404, got %d", status)
		}
	})
}

func TestProxyMTGStocks_NetworkError(t *testing.T) {
	withErrClient(func() {
		_, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/1")
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if status != 502 {
			t.Errorf("want status 502, got %d", status)
		}
	})
}

// ── fetchSealedPrice ───────────────────────────────────────────────────────────

func TestFetchSealedPrice_PrefersMarket(t *testing.T) {
	// Real Core Set 2020 snapshot: TCGplayer showed $6.55 market.
	withMockClient(200, `{"latestPrice":{"low":4.50,"average":6.74,"market":6.62}}`, func() {
		price, ok := fetchSealedPrice(1)
		if !ok {
			t.Fatal("want ok=true")
		}
		if price != 6.62 {
			t.Errorf("want market 6.62, got %v", price)
		}
	})
}

func TestFetchSealedPrice_FallsBackToAverage(t *testing.T) {
	withMockClient(200, `{"latestPrice":{"low":4.50,"market":0,"average":12.34}}`, func() {
		price, ok := fetchSealedPrice(1)
		if !ok {
			t.Fatal("want ok=true")
		}
		if price != 12.34 {
			t.Errorf("want 12.34, got %v", price)
		}
	})
}

func TestFetchSealedPrice_FallsBackToLow(t *testing.T) {
	withMockClient(200, `{"latestPrice":{"low":4.50,"market":null,"average":null}}`, func() {
		price, ok := fetchSealedPrice(1)
		if !ok {
			t.Fatal("want ok=true")
		}
		if price != 4.50 {
			t.Errorf("want 4.50, got %v", price)
		}
	})
}

func TestFetchSealedPrice_Non200(t *testing.T) {
	withMockClient(404, `{}`, func() {
		_, ok := fetchSealedPrice(9999)
		if ok {
			t.Error("want ok=false for non-200 status")
		}
	})
}

func TestFetchSealedPrice_NetworkError(t *testing.T) {
	withErrClient(func() {
		_, ok := fetchSealedPrice(1)
		if ok {
			t.Error("want ok=false on network error")
		}
	})
}

func TestFetchSealedPrice_BadJSON(t *testing.T) {
	withMockClient(200, `not json`, func() {
		_, ok := fetchSealedPrice(1)
		if ok {
			t.Error("want ok=false for bad JSON")
		}
	})
}

func TestFetchSealedPrice_NilLatestPrice(t *testing.T) {
	withMockClient(200, `{"latestPrice":null}`, func() {
		_, ok := fetchSealedPrice(1)
		if ok {
			t.Error("want ok=false when latestPrice is null")
		}
	})
}

func TestFetchSealedPrice_AllZero(t *testing.T) {
	withMockClient(200, `{"latestPrice":{"average":0,"market":0,"low":0}}`, func() {
		_, ok := fetchSealedPrice(1)
		if ok {
			t.Error("want ok=false when all prices are 0")
		}
	})
}

// ── Price handler ──────────────────────────────────────────────────────────────

func TestPrice_Handler_Success(t *testing.T) {
	respBody := `{"id":1,"latestPrice":{"average":5.00}}`
	withMockClient(200, respBody, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/price/1", nil)
		req.SetPathValue("mtgstocksId", "1")
		rr := httptest.NewRecorder()
		Price(rr, req)

		if rr.Code != 200 {
			t.Errorf("want 200, got %d", rr.Code)
		}
		if rr.Body.String() != respBody {
			t.Errorf("body mismatch: want %q, got %q", respBody, rr.Body.String())
		}
	})
}

func TestPrice_Handler_ProxiesNon200(t *testing.T) {
	withMockClient(404, `{}`, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/price/9999", nil)
		req.SetPathValue("mtgstocksId", "9999")
		rr := httptest.NewRecorder()
		Price(rr, req)

		if rr.Code != 404 {
			t.Errorf("want 404, got %d", rr.Code)
		}
	})
}

func TestPrice_Handler_NetworkError(t *testing.T) {
	withErrClient(func() {
		req := httptest.NewRequest(http.MethodGet, "/api/price/1", nil)
		req.SetPathValue("mtgstocksId", "1")
		rr := httptest.NewRecorder()
		Price(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Errorf("want 502, got %d", rr.Code)
		}
	})
}

func TestNewMTGStocksClient_Proxy(t *testing.T) {
	if c := newMTGStocksClient(""); c.Transport != nil {
		t.Error("no proxy: want default transport")
	}
	if c := newMTGStocksClient("::not a url"); c.Transport != nil {
		t.Error("invalid proxy: want default transport")
	}
	c := newMTGStocksClient("http://10.8.0.2:8888")
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("want proxying transport")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.mtgstocks.com/sealed", nil)
	if u, err := tr.Proxy(req); err != nil || u.Host != "10.8.0.2:8888" {
		t.Errorf("proxy for request: %v %v", u, err)
	}
}

func TestProxyMTGStocks_BacksOffAfterFailure(t *testing.T) {
	blocked := &blockedCounter{status: http.StatusForbidden}
	withTransport(blocked, func() {
		for i := 0; i < 3; i++ {
			_, _, _ = proxyMTGStocks("https://api.mtgstocks.com/sealed/1")
		}
		if n := blocked.calls.Load(); n != 1 {
			t.Errorf("want 1 request during backoff, got %d", n)
		}
		if _, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/1"); err != errMTGStocksBackoff || status != http.StatusServiceUnavailable {
			t.Errorf("want backoff error, got %d %v", status, err)
		}
		// Prices still come from the saved catalog meanwhile.
		catalogMu.Lock()
		currentCatalog, lastLiveAttempt = &catalog{prices: map[int]float64{1: 2.5}}, time.Now()
		catalogMu.Unlock()
		defer resetProductsCache()
		if price, ok := fetchSealedPrice(1); !ok || price != 2.5 {
			t.Errorf("want cached 2.5, got %v %v", price, ok)
		}
	})
}

func TestProxyMTGStocks_NotFoundIsNotAnOutage(t *testing.T) {
	notFound := &blockedCounter{status: http.StatusNotFound}
	withTransport(notFound, func() {
		_, _, _ = proxyMTGStocks("https://api.mtgstocks.com/sealed/1")
		_, _, _ = proxyMTGStocks("https://api.mtgstocks.com/sealed/2")
		if n := notFound.calls.Load(); n != 2 {
			t.Errorf("404 must not trigger backoff: want 2 requests, got %d", n)
		}
	})
}

func TestProxyMTGStocks_SuccessClearsBackoff(t *testing.T) {
	recordMTGStocksResult(0, fmt.Errorf("down"))
	recordMTGStocksResult(http.StatusOK, nil)
	mtgstocksMu.Lock()
	defer mtgstocksMu.Unlock()
	if !mtgstocksDownUntil.IsZero() {
		t.Error("success should clear the backoff")
	}
}

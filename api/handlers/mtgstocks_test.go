package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	defer func() { mtgstocksClient = orig }()
	f()
}

func withErrClient(f func()) {
	orig := mtgstocksClient
	mtgstocksClient = &http.Client{Transport: errTransport{}}
	defer func() { mtgstocksClient = orig }()
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

func TestFetchSealedPrice_Average(t *testing.T) {
	withMockClient(200, `{"latestPrice":{"average":12.34,"market":10.00}}`, func() {
		price, ok := fetchSealedPrice(1)
		if !ok {
			t.Fatal("want ok=true")
		}
		if price != 12.34 {
			t.Errorf("want 12.34, got %v", price)
		}
	})
}

func TestFetchSealedPrice_FallsBackToMarket(t *testing.T) {
	withMockClient(200, `{"latestPrice":{"average":null,"market":9.99}}`, func() {
		price, ok := fetchSealedPrice(1)
		if !ok {
			t.Fatal("want ok=true")
		}
		if price != 9.99 {
			t.Errorf("want 9.99, got %v", price)
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

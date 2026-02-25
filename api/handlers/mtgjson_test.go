package handlers

import (
	"net/http"
	"testing"
	"time"
)

const sampleSetList = `{
  "data": [
    {
      "code": "ZNR",
      "sealedProduct": [
        {"name": "Draft Booster",     "cardCount": 15, "category": "booster_pack"},
        {"name": "Collector Booster", "cardCount": 15, "category": "booster_pack"},
        {"name": "Bundle",            "cardCount": 10, "category": "booster_box"}
      ]
    },
    {
      "code": "MAT",
      "sealedProduct": [
        {"name": "Epilogue Booster", "cardCount": 5, "category": "booster_pack"}
      ]
    },
    {
      "code": "SML",
      "sealedProduct": [
        {"name": "Mini Booster", "cardCount": 8, "category": "booster_pack"}
      ]
    }
  ]
}`

type countingTransport struct {
	inner *stubTransport
	count *int
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*c.count++
	return c.inner.RoundTrip(r)
}

// resetCpp resets the in-memory cache state and returns a restore function.
func resetCpp(t *testing.T, client *http.Client) func() {
	t.Helper()
	origClient := mtgjsonClient
	origCache := cppCache
	origTime := cppCachedAt
	mtgjsonClient = client
	cppCache = nil
	cppCachedAt = time.Time{}
	return func() {
		mtgjsonClient = origClient
		cppCache = origCache
		cppCachedAt = origTime
	}
}

func sampleClient() *http.Client {
	return &http.Client{Transport: &stubTransport{statusCode: 200, body: sampleSetList}}
}

// ── lookupCardsPerPack ─────────────────────────────────────────────────────────

func TestLookupCardsPerPack_StandardPack(t *testing.T) {
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("ZNR", "Draft Booster"); got != 15 {
		t.Errorf("want 15, got %d", got)
	}
}

func TestLookupCardsPerPack_SubstringMatch(t *testing.T) {
	// MTGJSON name "epilogue booster" is a substring of the full MTGStocks
	// product name stored as productType.
	defer resetCpp(t, sampleClient())()
	got := lookupCardsPerPack("MAT", "March of the Machine: The Aftermath Epilogue Booster Pack")
	if got != 5 {
		t.Errorf("want 5, got %d", got)
	}
}

func TestLookupCardsPerPack_ExactMatch(t *testing.T) {
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("MAT", "Epilogue Booster"); got != 5 {
		t.Errorf("want 5, got %d", got)
	}
}

func TestLookupCardsPerPack_SingleProduct_NoNameMatch(t *testing.T) {
	// SML has only one booster product; even with a name mismatch, return its count.
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("SML", "Unknown Booster"); got != 8 {
		t.Errorf("want 8 (single-product fallback), got %d", got)
	}
}

func TestLookupCardsPerPack_UnknownSet(t *testing.T) {
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("XYZ", "Draft Booster"); got != 15 {
		t.Errorf("want 15 for unknown set, got %d", got)
	}
}

func TestLookupCardsPerPack_MultipleProducts_NoMatch(t *testing.T) {
	// ZNR has two booster products; an unrecognised type returns 15.
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("ZNR", "Some Unknown Booster"); got != 15 {
		t.Errorf("want 15 when no name matches and multiple products, got %d", got)
	}
}

func TestLookupCardsPerPack_CaseInsensitive(t *testing.T) {
	defer resetCpp(t, sampleClient())()
	if got := lookupCardsPerPack("mat", "EPILOGUE BOOSTER"); got != 5 {
		t.Errorf("want 5 for lowercase code / uppercase type, got %d", got)
	}
}

func TestLookupCardsPerPack_NetworkError_Returns15(t *testing.T) {
	defer resetCpp(t, &http.Client{Transport: errTransport{}})()
	if got := lookupCardsPerPack("MAT", "Epilogue Booster"); got != 15 {
		t.Errorf("want 15 on network error, got %d", got)
	}
}

func TestLookupCardsPerPack_BadJSON_Returns15(t *testing.T) {
	defer resetCpp(t, &http.Client{Transport: &stubTransport{statusCode: 200, body: "not json"}})()
	if got := lookupCardsPerPack("MAT", "Epilogue Booster"); got != 15 {
		t.Errorf("want 15 on bad JSON, got %d", got)
	}
}

func TestLookupCardsPerPack_Non200_Returns15(t *testing.T) {
	defer resetCpp(t, &http.Client{Transport: &stubTransport{statusCode: 503, body: ""}})()
	if got := lookupCardsPerPack("MAT", "Epilogue Booster"); got != 15 {
		t.Errorf("want 15 on non-200 response, got %d", got)
	}
}

func TestLookupCardsPerPack_CacheReused(t *testing.T) {
	n := 0
	client := &http.Client{Transport: countingTransport{
		inner: &stubTransport{statusCode: 200, body: sampleSetList},
		count: &n,
	}}
	defer resetCpp(t, client)()

	lookupCardsPerPack("MAT", "Epilogue Booster")
	lookupCardsPerPack("ZNR", "Draft Booster")
	if n != 1 {
		t.Errorf("want exactly 1 HTTP call for two lookups, got %d", n)
	}
}

func TestLookupCardsPerPack_CacheExpiry(t *testing.T) {
	n := 0
	client := &http.Client{Transport: countingTransport{
		inner: &stubTransport{statusCode: 200, body: sampleSetList},
		count: &n,
	}}
	origClient := mtgjsonClient
	origCache := cppCache
	origTime := cppCachedAt
	mtgjsonClient = client
	cppCache = make(map[string][]cppEntry) // non-nil but stale
	cppCachedAt = time.Now().Add(-(mtgjsonCacheTTL + time.Minute))
	defer func() {
		mtgjsonClient = origClient
		cppCache = origCache
		cppCachedAt = origTime
	}()

	lookupCardsPerPack("MAT", "Epilogue Booster")
	if n != 1 {
		t.Errorf("want 1 HTTP call after cache expiry, got %d", n)
	}
}

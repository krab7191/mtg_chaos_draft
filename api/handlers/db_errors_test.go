package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"mtg-chaos-draft/db"
	mw "mtg-chaos-draft/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

// closedPool returns a pool whose every query fails, to exercise the
// "database is down" path of each handler.
func closedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	return pool
}

// TestHandlers_DatabaseDown: every endpoint answers 500 (not a panic, not a
// misleading 2xx/4xx) when Postgres fails.
func TestHandlers_DatabaseDown(t *testing.T) {
	pool := closedPool(t)
	col := NewCollectionHandler(pool)
	drafts := NewDraftHandler(pool)
	purchases := NewPurchaseHandler(pool)
	sales := NewSaleHandler(pool)
	settings := NewSettingsHandler(pool)
	catalog := &CatalogHandler{pool: pool}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		id      string
		body    string
	}{
		{"collection list", col.List, "", ""},
		{"collection add", col.Add, "", `{"mtgstocksId":1,"name":"P","setName":"S","productType":"Play Booster","quantity":1,"weight":1}`},
		{"collection update", col.Update, "1", `{"quantity":2}`},
		{"collection link price", col.LinkPrice, "1", `{"mtgstocksId":1}`},
		{"collection refresh price", col.RefreshPrice, "1", ""},
		{"collection delete", col.Delete, "1", ""},
		{"drafts create", drafts.Create, "", `{"podSize":2,"picks":[{"setName":"S","productType":"Play Booster"},{"setName":"S","productType":"Play Booster"},{"setName":"S","productType":"Play Booster"},{"setName":"S","productType":"Play Booster"},{"setName":"S","productType":"Play Booster"},{"setName":"S","productType":"Play Booster"}]}`},
		{"drafts list", drafts.List, "", ""},
		{"drafts delete", drafts.Delete, "1", ""},
		{"drafts approve", drafts.Approve, "1", ""},
		{"purchases list", purchases.List, "", ""},
		{"purchases create", purchases.Create, "", `{"packId":1,"quantity":1,"paidPrice":1,"purchasedOn":"2026-01-01"}`},
		{"purchases update", purchases.Update, "1", `{"quantity":1,"paidPrice":1,"purchasedOn":"2026-01-01"}`},
		{"purchases delete", purchases.Delete, "1", ""},
		{"sales list", sales.List, "", ""},
		{"sales create", sales.Create, "", `{"packId":1,"quantity":1,"salePrice":1,"soldOn":"2026-01-01"}`},
		{"sales update", sales.Update, "1", `{"packId":1,"quantity":1,"salePrice":1,"soldOn":"2026-01-01"}`},
		{"sales delete", sales.Delete, "1", ""},
		{"draft collected", sales.SetDraftCollected, "1", `{"amount":10}`},
		{"settings get", settings.Get, "", ""},
		{"settings update", settings.Update, "", `{"priceSensitivity":1,"scarcitySensitivity":1,"priceCap":0,"priceFloor":0,"quantityCap":0,"packWeights":{}}`},
		{"catalog upload", catalog.Upload, "", fakeSetsJSON},
	}

	withMockClient(200, `{"latestPrice":{"market":5}}`, func() {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
				if tc.id != "" {
					req.SetPathValue("id", tc.id)
				}
				req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, &db.User{ID: 1, Role: "admin"}))
				rr := httptest.NewRecorder()
				tc.handler(rr, req)
				if rr.Code < 500 {
					t.Errorf("want 5xx, got %d: %s", rr.Code, rr.Body)
				}
			})
		}
	})
}

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The MTGStocks sealed catalog backs search, cheapest packs and pricing.
// Sources, in order: memory (refreshed live at most hourly), a live fetch,
// then the last good copy in Postgres. Postgres is also seeded by admin
// uploads, so things keep working while MTGStocks refuses our server.

// catalogLiveInterval is how long to wait between live fetch attempts,
// successful or not, so a blocked server doesn't hammer MTGStocks.
const catalogLiveInterval = time.Hour

// maxCatalogUpload bounds an uploaded /sealed snapshot (~1MB today).
const maxCatalogUpload = 32 << 20

type catalog struct {
	products  []SearchResult  // one per set + pack type, for search
	prices    map[int]float64 // every priced product by MTGStocks id
	fetchedAt time.Time
	source    string // "live" or "upload"
}

var (
	catalogMu       sync.Mutex
	currentCatalog  *catalog
	lastLiveAttempt time.Time
	// catalogPool persists the catalog; nil disables persistence (unit tests).
	catalogPool *pgxpool.Pool
)

// getCatalog returns the freshest catalog available, or an error when there
// is none at all.
func getCatalog() (*catalog, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()

	if currentCatalog != nil && time.Since(lastLiveAttempt) < catalogLiveInterval {
		return currentCatalog, nil
	}
	lastLiveAttempt = time.Now()

	live, err := fetchLiveCatalog()
	if err == nil {
		currentCatalog = live
		fmt.Printf("mtgstocks: loaded %d sealed products\n", len(live.products))
		return live, nil
	}
	fmt.Printf("mtgstocks: catalog refresh failed: %v\n", err)

	if currentCatalog == nil {
		if saved, loadErr := loadSavedCatalog(); loadErr == nil {
			currentCatalog = saved
			fmt.Printf("mtgstocks: using saved %s catalog from %s\n", saved.source, saved.fetchedAt.Format(time.RFC3339))
		} else if !errors.Is(loadErr, db.ErrNoCatalog) {
			fmt.Printf("mtgstocks: loading saved catalog: %v\n", loadErr)
		}
	}
	if currentCatalog != nil {
		return currentCatalog, nil
	}
	return nil, err
}

// getMTGStocksProducts returns the search catalog.
func getMTGStocksProducts() ([]SearchResult, error) {
	c, err := getCatalog()
	if err != nil {
		return nil, err
	}
	return c.products, nil
}

// cachedSealedPrice is the catalog's price for a product, for when a live
// per-product lookup fails.
func cachedSealedPrice(mtgstocksID int) (float64, bool) {
	c, err := getCatalog()
	if err != nil {
		return 0, false
	}
	price, ok := c.prices[mtgstocksID]
	return price, ok
}

func fetchLiveCatalog() (*catalog, error) {
	body, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed")
	if err != nil {
		return nil, fmt.Errorf("mtgstocks fetch: %w", err)
	}
	if status != http.StatusOK {
		snippet := body
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("mtgstocks returned %d: %s", status, snippet)
	}
	c, err := parseCatalog(body)
	if err != nil {
		return nil, err
	}
	c.fetchedAt, c.source = time.Now(), "live"
	saveCatalog(body, c)
	return c, nil
}

func loadSavedCatalog() (*catalog, error) {
	if catalogPool == nil {
		return nil, db.ErrNoCatalog
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	saved, err := db.LoadCatalog(ctx, catalogPool)
	if err != nil {
		return nil, err
	}
	c, err := parseCatalog(saved.Body)
	if err != nil {
		return nil, fmt.Errorf("saved catalog: %w", err)
	}
	c.fetchedAt, c.source = saved.FetchedAt, saved.Source
	return c, nil
}

// saveCatalog persists a good catalog; failures are logged, not fatal.
func saveCatalog(raw []byte, c *catalog) {
	if catalogPool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.SaveCatalog(ctx, catalogPool, db.SavedCatalog{Body: raw, FetchedAt: c.fetchedAt, Source: c.source}); err != nil {
		log.Printf("mtgstocks: saving catalog: %v", err)
	}
}

// parseCatalog turns a raw MTGStocks /sealed response into a catalog.
func parseCatalog(raw []byte) (*catalog, error) {
	var sets []msSet
	if err := json.Unmarshal(raw, &sets); err != nil {
		return nil, fmt.Errorf("mtgstocks decode: %w", err)
	}

	c := &catalog{prices: make(map[int]float64)}
	type seenKey struct{ setCode, productType string }
	seen := make(map[seenKey]bool)

	addProduct := func(s msSet, p msProduct) {
		productType, ok := deriveProductType(p)
		if !ok {
			return
		}
		key := seenKey{s.Abbreviation, productType}
		if seen[key] {
			return
		}
		seen[key] = true
		result := SearchResult{
			MTGStocksID: p.ID,
			Name:        p.Name,
			SetName:     s.Name,
			SetCode:     s.Abbreviation,
			ReleasedAt:  s.Date,
			ProductType: productType,
		}
		if price, ok := c.prices[p.ID]; ok {
			result.MarketPrice = &price
		}
		c.products = append(c.products, result)
	}

	for _, s := range sets {
		for _, p := range s.Products {
			if p.LatestPrice != nil {
				if price, ok := pickPrice(p.LatestPrice.Low, p.LatestPrice.Market, p.LatestPrice.Average); ok {
					c.prices[p.ID] = price
				}
			}
		}
		// Pass 1: explicitly-typed products win deduplication
		for _, p := range s.Products {
			if p.Type == nil {
				continue
			}
			if _, ok := knownPackTypes[*p.Type]; !ok {
				continue
			}
			addProduct(s, p)
		}
		// Pass 2: null/unknown-typed products only fill slots not already taken
		for _, p := range s.Products {
			if p.Type != nil {
				if _, ok := knownPackTypes[*p.Type]; ok {
					continue
				}
			}
			addProduct(s, p)
		}
	}
	return c, nil
}

// CatalogHandler lets admins see and seed the saved MTGStocks catalog.
type CatalogHandler struct {
	pool *pgxpool.Pool
}

// UseCatalogStore persists the catalog to pool and falls back to it.
func UseCatalogStore(pool *pgxpool.Pool) {
	catalogMu.Lock()
	catalogPool = pool
	catalogMu.Unlock()
}

// NewCatalogHandler also enables the catalog store (see UseCatalogStore).
func NewCatalogHandler(pool *pgxpool.Pool) *CatalogHandler {
	UseCatalogStore(pool)
	return &CatalogHandler{pool: pool}
}

type catalogStatus struct {
	Available bool       `json:"available"`
	FetchedAt *time.Time `json:"fetchedAt"`
	Source    string     `json:"source"`
	Products  int        `json:"products"`
}

func statusOf(c *catalog) catalogStatus {
	if c == nil {
		return catalogStatus{}
	}
	t := c.fetchedAt
	return catalogStatus{Available: true, FetchedAt: &t, Source: c.source, Products: len(c.products)}
}

// Status reports where the catalog came from and how old it is.
func (h *CatalogHandler) Status(w http.ResponseWriter, _ *http.Request) {
	c, _ := getCatalog()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusOf(c))
}

// Upload stores an admin-provided MTGStocks /sealed snapshot and applies its
// prices to the collection.
func (h *CatalogHandler) Upload(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCatalogUpload))
	if err != nil {
		http.Error(w, "catalog file too large", http.StatusRequestEntityTooLarge)
		return
	}
	c, err := parseCatalog(raw)
	if err != nil || len(c.products) == 0 {
		http.Error(w, "not an MTGStocks sealed catalog — upload the JSON from api.mtgstocks.com/sealed", http.StatusBadRequest)
		return
	}
	c.fetchedAt, c.source = time.Now(), "upload"

	if err := db.SaveCatalog(r.Context(), h.pool, db.SavedCatalog{Body: raw, FetchedAt: c.fetchedAt, Source: c.source}); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	catalogMu.Lock()
	currentCatalog = c
	catalogMu.Unlock()

	if err := db.BulkUpdatePrices(r.Context(), h.pool, c.prices); err != nil {
		log.Printf("catalog upload: updating collection prices: %v", err)
	}
	log.Printf("catalog upload: %d sealed products", len(c.products))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusOf(c))
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TCGplayer product pages for single packs, so "TCGplayer" links go straight
// to the pack instead of a search full of prerelease kits. MTGStocks has each
// product's page in its per-product data; the hourly backfill fetches the
// missing ones, spaced like the history backfill, once each.

var (
	tcgLinksMu sync.RWMutex
	tcgLinks   = map[int]string{} // by MTGStocks id; "" = none
)

// tcgLink is a product's TCGplayer page, or "" if unknown.
func tcgLink(id int) string {
	tcgLinksMu.RLock()
	defer tcgLinksMu.RUnlock()
	return tcgLinks[id]
}

// LoadProductLinks reads the stored links into memory (at startup).
func LoadProductLinks(ctx context.Context, pool *pgxpool.Pool) error {
	links, err := db.ProductLinks(ctx, pool)
	if err != nil {
		return err
	}
	tcgLinksMu.Lock()
	tcgLinks = links
	tcgLinksMu.Unlock()
	return nil
}

// directTCGURL unwraps MTGStocks' referral link
// (https://partner.tcgplayer.com/…?u=<page>) to the plain product page.
func directTCGURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if inner := u.Query().Get("u"); inner != "" {
		if iu, err := url.Parse(inner); err == nil && strings.HasSuffix(iu.Host, "tcgplayer.com") {
			return iu.Scheme + "://" + iu.Host + iu.Path
		}
		return ""
	}
	if strings.HasSuffix(u.Host, "tcgplayer.com") && strings.HasPrefix(u.Path, "/product/") {
		return u.Scheme + "://" + u.Host + u.Path
	}
	return ""
}

// fetchTCGURL asks MTGStocks for a product's TCGplayer page.
func fetchTCGURL(id int) (string, error) {
	body, status, err := proxyMTGStocks(fmt.Sprintf("https://api.mtgstocks.com/sealed/%d", id))
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", nil
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("mtgstocks returned %d", status)
	}
	var data struct {
		TCGURL string `json:"tcgUrl"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("decode product: %w", err)
	}
	return directTCGURL(data.TCGURL), nil
}

// backfillTCGLinks fetches the TCGplayer page of every single pack that
// doesn't have one yet. Stops at the first failure; the next run resumes.
func backfillTCGLinks(ctx context.Context, pool *pgxpool.Pool, c *catalog) (int, error) {
	n := 0
	for _, p := range c.products {
		tcgLinksMu.RLock()
		_, known := tcgLinks[p.MTGStocksID]
		tcgLinksMu.RUnlock()
		if known {
			continue
		}
		select {
		case <-time.After(backfillDelay):
		case <-ctx.Done():
			return n, ctx.Err()
		}
		link, err := fetchTCGURL(p.MTGStocksID)
		if err != nil {
			return n, fmt.Errorf("TCGplayer link for %s %s: %w", p.SetName, p.ProductType, err)
		}
		if err := db.SetProductLink(ctx, pool, p.MTGStocksID, link); err != nil {
			return n, err
		}
		tcgLinksMu.Lock()
		tcgLinks[p.MTGStocksID] = link
		tcgLinksMu.Unlock()
		n++
	}
	if n > 0 {
		log.Printf("product links: fetched %d TCGplayer links", n)
	}
	return n, nil
}

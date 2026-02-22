package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sahilm/fuzzy"
)

type msProduct struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type        *string `json:"type"`
	LatestPrice *struct {
		Average *float64 `json:"average"`
		Market  *float64 `json:"market"`
	} `json:"latestPrice"`
}

type msSet struct {
	Name         string      `json:"name"`
	Abbreviation string      `json:"abbreviation"`
	Date         string      `json:"date"`
	Products     []msProduct `json:"products"`
}

type SearchResult struct {
	MTGStocksID int      `json:"mtgstocksId"`
	Name        string   `json:"name"`
	SetName     string   `json:"setName"`
	SetCode     string   `json:"setCode"`
	ReleasedAt  string   `json:"releasedAt"`
	ProductType string   `json:"productType"`
	MarketPrice *float64 `json:"marketPrice"`
}

var (
	productsCache     []SearchResult
	productsCacheTime time.Time
	productsCacheMu   sync.RWMutex
)

var knownPackTypes = map[string]string{
	"draft_booster":     "Draft Booster",
	"collector_booster": "Collector Booster",
	"set_booster":       "Set Booster",
	"play_booster":      "Play Booster",
}

var excludePackTypes = map[string]bool{
	"case":       true,
	"bundle":     true,
	"boosterbox": true,
}

func deriveProductType(p msProduct) (string, bool) {
	if p.Type != nil {
		if label, ok := knownPackTypes[*p.Type]; ok {
			return label, true
		}
		if excludePackTypes[*p.Type] {
			return "", false
		}
	}

	// null type — include only if name suggests it's a booster pack
	nameLower := strings.ToLower(p.Name)
	if strings.Contains(nameLower, "booster pack") {
		switch {
		case strings.Contains(nameLower, "jumpstart"):
			return "Jumpstart Booster", true
		case strings.Contains(nameLower, "play"):
			return "Play Booster", true
		case strings.Contains(nameLower, "set"):
			return "Set Booster", true
		case strings.Contains(nameLower, "draft"):
			return "Draft Booster", true
		case strings.Contains(nameLower, "collector"):
			return "Collector Booster", true
		default:
			return p.Name, true
		}
	}
	return "", false
}

func getMTGStocksProducts() ([]SearchResult, error) {
	productsCacheMu.RLock()
	if productsCache != nil && time.Since(productsCacheTime) < time.Hour {
		p := productsCache
		productsCacheMu.RUnlock()
		return p, nil
	}
	productsCacheMu.RUnlock()

	body, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed")
	if err != nil {
		return nil, fmt.Errorf("mtgstocks fetch: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("mtgstocks returned %d", status)
	}

	var sets []msSet
	if err := json.Unmarshal(body, &sets); err != nil {
		return nil, fmt.Errorf("mtgstocks decode: %w", err)
	}

	var products []SearchResult
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
		if p.LatestPrice != nil {
			// Prefer average price — matches what MTGStocks UI displays
			if p.LatestPrice.Average != nil && *p.LatestPrice.Average > 0 {
				result.MarketPrice = p.LatestPrice.Average
			} else if p.LatestPrice.Market != nil && *p.LatestPrice.Market > 0 {
				result.MarketPrice = p.LatestPrice.Market
			}
		}
		products = append(products, result)
	}

	for _, s := range sets {
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

	fmt.Printf("mtgstocks: loaded %d sealed products from %d sets\n", len(products), len(sets))

	productsCacheMu.Lock()
	productsCache = products
	productsCacheTime = time.Now()
	productsCacheMu.Unlock()

	return products, nil
}

func Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}

	products, err := getMTGStocksProducts()
	if err != nil {
		http.Error(w, "search error", http.StatusBadGateway)
		return
	}

	// Build search strings: "SetName ProductName" for matching
	searchStrings := make([]string, len(products))
	for i, p := range products {
		searchStrings[i] = p.SetName + " " + p.Name
	}

	var results []SearchResult
	qLower := strings.ToLower(q)

	if len(q) < 5 {
		for i, s := range searchStrings {
			if strings.Contains(strings.ToLower(s), qLower) {
				results = append(results, products[i])
			}
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].ReleasedAt > results[j].ReleasedAt
		})
	} else {
		matches := fuzzy.Find(q, searchStrings)
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].Score != matches[j].Score {
				return matches[i].Score > matches[j].Score
			}
			return products[matches[i].Index].ReleasedAt > products[matches[j].Index].ReleasedAt
		})
		for _, m := range matches {
			results = append(results, products[m.Index])
		}
	}

	if len(results) > 30 {
		results = results[:30]
	}
	if results == nil {
		results = []SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	mtgjsonSetListURL = "https://mtgjson.com/api/v5/SetList.json"
	mtgjsonCacheTTL   = 7 * 24 * time.Hour
)

var mtgjsonClient = &http.Client{Timeout: 30 * time.Second}

// cppEntry holds a single booster_pack product from MTGJSON for one set.
type cppEntry struct {
	name      string // lowercase MTGJSON product name
	cardCount int
}

var (
	cppCache    map[string][]cppEntry // lowercase set code → booster_pack entries
	cppCachedAt time.Time
	cppMu       sync.RWMutex
)

func fetchMtgjsonSetList() (map[string][]cppEntry, error) {
	req, err := http.NewRequest("GET", mtgjsonSetListURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := mtgjsonClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mtgjson fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mtgjson returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mtgjson read: %w", err)
	}

	var response struct {
		Data []struct {
			Code          string `json:"code"`
			SealedProduct []struct {
				Name      string `json:"name"`
				CardCount int    `json:"cardCount"`
				Category  string `json:"category"`
			} `json:"sealedProduct"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("mtgjson decode: %w", err)
	}

	result := make(map[string][]cppEntry)
	for _, s := range response.Data {
		code := strings.ToLower(s.Code)
		for _, p := range s.SealedProduct {
			if p.Category != "booster_pack" || p.CardCount <= 0 {
				continue
			}
			result[code] = append(result[code], cppEntry{
				name:      strings.ToLower(p.Name),
				cardCount: p.CardCount,
			})
		}
	}
	return result, nil
}

func ensureCppCache() {
	cppMu.RLock()
	fresh := cppCache != nil && time.Since(cppCachedAt) < mtgjsonCacheTTL
	cppMu.RUnlock()
	if fresh {
		return
	}

	cache, err := fetchMtgjsonSetList()
	if err != nil {
		fmt.Printf("mtgjson: cache refresh failed: %v\n", err)
		// On first failure, set an empty cache so we don't hammer the API on every call.
		cppMu.Lock()
		if cppCache == nil {
			cppCache = make(map[string][]cppEntry)
			cppCachedAt = time.Now()
		}
		cppMu.Unlock()
		return
	}

	cppMu.Lock()
	cppCache = cache
	cppCachedAt = time.Now()
	cppMu.Unlock()
	fmt.Printf("mtgjson: cached %d sets\n", len(cache))
}

// lookupCardsPerPack returns the number of cards per booster for the given
// MTG set code and product type. Returns 15 if no unambiguous match is found.
func lookupCardsPerPack(setCode, productType string) int {
	ensureCppCache()

	cppMu.RLock()
	entries := cppCache[strings.ToLower(setCode)]
	cppMu.RUnlock()

	if len(entries) == 0 {
		return 15
	}

	ptLower := strings.ToLower(productType)
	for _, e := range entries {
		// Match if MTGJSON name is a substring of our productType ("epilogue booster"
		// matches "march of the machine: the aftermath epilogue booster pack"), or
		// our productType is a substring of the MTGJSON name (for exact/short types).
		if strings.Contains(ptLower, e.name) || strings.Contains(e.name, ptLower) {
			return e.cardCount
		}
	}

	// If only one booster product type exists for this set, use it unambiguously.
	if len(entries) == 1 {
		return entries[0].cardCount
	}

	return 15
}

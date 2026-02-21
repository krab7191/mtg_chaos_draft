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

type scryfallSet struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	SetType    string `json:"set_type"`
	ReleasedAt string `json:"released_at"`
	Digital    bool   `json:"digital"`
	IconSVGURI string `json:"icon_svg_uri"`
}

type scryfallSetsResponse struct {
	Data []scryfallSet `json:"data"`
}

type SearchResult struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	SetType      string   `json:"setType"`
	ReleasedAt   string   `json:"releasedAt"`
	IconURL      string   `json:"iconUrl"`
	ProductTypes []string `json:"productTypes"`
}

var (
	setsCache     []scryfallSet
	setsCacheTime time.Time
	setsCacheMu   sync.RWMutex
)

var (
	playBoosterDate      = time.Date(2024, 2, 9, 0, 0, 0, 0, time.UTC)
	setBoosterDate       = time.Date(2020, 9, 25, 0, 0, 0, 0, time.UTC)
	collectorBoosterDate = time.Date(2019, 10, 4, 0, 0, 0, 0, time.UTC)
	jumpstartEraStart    = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC) // Phyrexia: All Will Be One onwards
)

var draftableSetTypes = map[string]bool{
	"expansion":        true,
	"core":             true,
	"masters":          true,
	"draft_innovation": true,
	"starter":          true,
}

func getScryfallSets() ([]scryfallSet, error) {
	setsCacheMu.RLock()
	if setsCache != nil && time.Since(setsCacheTime) < time.Hour {
		sets := setsCache
		setsCacheMu.RUnlock()
		return sets, nil
	}
	setsCacheMu.RUnlock()

	req, err := http.NewRequest("GET", "https://api.scryfall.com/sets", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mtg-chaos-draft/1.0 (personal draft app)")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("scryfall error: %v\n", err)
		return nil, fmt.Errorf("scryfall request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("scryfall bad status: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("scryfall returned %d", resp.StatusCode)
	}

	var result scryfallSetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("scryfall decode: %w", err)
	}

	fmt.Printf("scryfall: fetched %d total sets\n", len(result.Data))

	var filtered []scryfallSet
	for _, s := range result.Data {
		if draftableSetTypes[s.SetType] && !s.Digital {
			filtered = append(filtered, s)
		}
	}

	fmt.Printf("scryfall: %d sets after filter\n", len(filtered))

	setsCacheMu.Lock()
	setsCache = filtered
	setsCacheTime = time.Now()
	setsCacheMu.Unlock()

	return filtered, nil
}

func productTypesForSet(s scryfallSet) []string {
	released, err := time.Parse("2006-01-02", s.ReleasedAt)
	if err != nil {
		return []string{"Draft Booster"}
	}

	var types []string

	// expansion, core, and draft_innovation (e.g. LotR, Commander Legends) can all
	// have Set Boosters and Jumpstart Packs when released in the right era.
	isMainSet := s.SetType == "expansion" || s.SetType == "core" || s.SetType == "draft_innovation"

	// masters sets can also have Collector Boosters (e.g. Double Masters 2022, MH3).
	hasCollector := isMainSet || s.SetType == "masters"

	if released.Before(playBoosterDate) {
		types = append(types, "Draft Booster")
		if isMainSet && !released.Before(setBoosterDate) {
			types = append(types, "Set Booster")
		}
		// Jumpstart Packs were sold alongside regular sets from ~early 2023 to Feb 2024
		if isMainSet && !released.Before(jumpstartEraStart) {
			types = append(types, "Jumpstart Booster")
		}
	} else {
		types = append(types, "Play Booster")
	}

	if hasCollector && !released.Before(collectorBoosterDate) {
		types = append(types, "Collector Booster")
	}

	return types
}

func Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	sets, err := getScryfallSets()
	if err != nil {
		http.Error(w, "scryfall error", http.StatusBadGateway)
		return
	}

	// Build name list for fuzzy matching (index matches sets slice)
	names := make([]string, len(sets))
	for i, s := range sets {
		names[i] = s.Name
	}

	var results []SearchResult
	qLower := strings.ToLower(q)

	if len(q) < 5 {
		// Short query: substring/prefix only — fuzzy is too permissive
		for _, s := range sets {
			if strings.Contains(strings.ToLower(s.Name), qLower) || strings.EqualFold(s.Code, q) {
				results = append(results, SearchResult{
					Code:         s.Code,
					Name:         s.Name,
					SetType:      s.SetType,
					ReleasedAt:   s.ReleasedAt,
					IconURL:      s.IconSVGURI,
					ProductTypes: productTypesForSet(s),
				})
			}
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].ReleasedAt > results[j].ReleasedAt
		})
	} else {
		// Longer query: use fuzzy matching
		matches := fuzzy.Find(q, names)
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].Score != matches[j].Score {
				return matches[i].Score > matches[j].Score
			}
			return sets[matches[i].Index].ReleasedAt > sets[matches[j].Index].ReleasedAt
		})
		for _, m := range matches {
			s := sets[m.Index]
			results = append(results, SearchResult{
				Code:         s.Code,
				Name:         s.Name,
				SetType:      s.SetType,
				ReleasedAt:   s.ReleasedAt,
				IconURL:      s.IconSVGURI,
				ProductTypes: productTypesForSet(s),
			})
		}
	}

	if len(results) > 8 {
		results = results[:8]
	}
	if results == nil {
		results = []SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

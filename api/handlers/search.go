package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/sahilm/fuzzy"
)

type msProduct struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type        *string `json:"type"`
	LatestPrice *struct {
		Low     *float64 `json:"low"`
		Market  *float64 `json:"market"`
		Average *float64 `json:"average"`
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
	// PacksPerBox is set only for booster boxes.
	PacksPerBox int `json:"packsPerBox,omitempty"`
	// TCGURL is the product's TCGplayer page, where known (cheapest packs).
	TCGURL string `json:"tcgUrl,omitempty"`
	// NoSales: MarketPrice is only the average asking price (no recent
	// sales), so it says little about what the product is worth.
	NoSales bool `json:"noSales,omitempty"`
	// PriceDoubt explains why MarketPrice looks wrong (box and pack prices
	// disagree); such products raise no alerts or deals.
	PriceDoubt string `json:"priceDoubt,omitempty"`
}

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

// notSinglePack matches names of products that hold several packs or aren't
// draft boosters, e.g. "Theme Booster Pack [Red]", "Collector Booster Pack
// Display", "3x Booster Packs (draft set)", "Jumpstart Booster Pack (2-Pack)".
var notSinglePack = regexp.MustCompile(
	`\b(theme|display|box|case|bundle|blister|battle|prerelease|omega|includes)\b|\bset of\b|\b\d+x\b|\d+-(pack|booster)\b|\bbooster packs\b`)

// plainBoosterNames are untyped single-pack names MTGStocks uses for newer
// sets ("Play Booster" rather than "Play Booster Pack").
var plainBoosterNames = map[string]bool{
	"play booster": true, "collector booster": true, "set booster": true,
	"draft booster": true, "jumpstart booster": true,
}

func isSingleBoosterName(nameLower string) bool {
	if notSinglePack.MatchString(nameLower) {
		return false
	}
	if strings.Contains(nameLower, "booster pack") {
		return true
	}
	// Drop a "<Set> - " prefix and a "Sleeved" marker before matching.
	if i := strings.LastIndex(nameLower, " - "); i >= 0 {
		nameLower = nameLower[i+3:]
	}
	nameLower = strings.TrimPrefix(strings.TrimSpace(nameLower), "sleeved ")
	return plainBoosterNames[nameLower]
}

// words returns the set of whole words in s, so "display" never counts as
// "play" and "[Set of 5]" never as a Set Booster.
func words(s string) map[string]bool {
	w := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) }) {
		w[f] = true
	}
	return w
}

// sleevedLabel prefixes sleeved packs' type. They're big-box retail
// exclusives (Target, Walmart; the sleeve hides an anti-theft tag) that sell
// out within weeks of release: searchable, so a store find can be added to
// the collection, but left out of cheapest packs and price trends.
const sleevedLabel = "Sleeved "

// isSleeved reports whether a product is a sleeved retail pack.
func isSleeved(p SearchResult) bool { return strings.HasPrefix(p.ProductType, sleevedLabel) }

// isJumpstart reports whether a product is a Jumpstart pack or box: 20-card
// themed half-decks for casual play, not drafted. "Jumpstart Booster" in most
// sets; the original Jumpstart sets list plain "Booster Pack"/"Booster Box".
func isJumpstart(setName, productType string) bool {
	return strings.Contains(productType, "Jumpstart") || strings.HasPrefix(setName, "Jumpstart")
}

func deriveProductType(p msProduct) (string, bool) {
	nameLower := strings.ToLower(p.Name)
	w := words(nameLower)
	sleevedPrefix := ""
	if w["sleeved"] {
		sleevedPrefix = sleevedLabel
	}

	if p.Type != nil {
		if label, ok := knownPackTypes[*p.Type]; ok {
			return sleevedPrefix + label, true
		}
		if excludePackTypes[*p.Type] {
			return "", false
		}
	}

	// Untyped: include only names that say they're a single booster pack.
	if !isSingleBoosterName(nameLower) {
		return "", false
	}
	switch {
	case w["jumpstart"]:
		return sleevedPrefix + "Jumpstart Booster", true
	case w["play"]:
		return sleevedPrefix + "Play Booster", true
	case w["set"]:
		return sleevedPrefix + "Set Booster", true
	case w["draft"]:
		return sleevedPrefix + "Draft Booster", true
	case w["collector"], w["collectors"]:
		return sleevedPrefix + "Collector Booster", true
	default:
		return p.Name, true
	}
}

func CheapestPacks(w http.ResponseWriter, r *http.Request) {
	products, err := getMTGStocksProducts()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}

	before := strings.TrimSpace(r.URL.Query().Get("before")) // YYYY-MM-DD or empty
	var beforeTime time.Time
	if before != "" {
		var err error
		beforeTime, err = time.Parse("2006-01-02", before)
		if err != nil {
			http.Error(w, "invalid before date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	var priced []SearchResult
	for _, p := range products {
		if p.MarketPrice == nil || *p.MarketPrice <= 0 || isSleeved(p) {
			continue
		}
		if !beforeTime.IsZero() {
			released, err := time.Parse("2006-01-02", p.ReleasedAt)
			if err != nil || !released.Before(beforeTime) {
				continue
			}
		}
		priced = append(priced, p)
	}
	sort.Slice(priced, func(i, j int) bool {
		return *priced[i].MarketPrice < *priced[j].MarketPrice
	})
	if len(priced) > 10 {
		priced = priced[:10]
	}
	for i := range priced {
		priced[i].TCGURL = tcgLink(priced[i].MTGStocksID)
	}
	if priced == nil {
		priced = []SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(priced)
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
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
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

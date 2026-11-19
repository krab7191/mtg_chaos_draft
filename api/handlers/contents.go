package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// What's in a pack and what it's worth: the latest card prices, pull rates
// and card names from the last daily EV run, kept in memory to break a
// pack's value down by slot and by card ("Check a pack" on Cheap Packs).

// premiumRatio: a pack costing this many times its contents is mostly a
// collectible premium. Recent Collector packs cost ~1.2–3× their contents,
// Play Boosters ~0.7–1.1×.
const premiumRatio = 3.0

type cardPrice struct{ normal, foil float64 }

var (
	contentsMu     sync.RWMutex
	contentsPacks  = map[int]mjBooster{} // by MTGStocks id
	contentsPrices = map[string]cardPrice{}
	contentsNames  = map[string]string{}
	contentsValue  = map[int]float64{} // each pack's contents value at the latest prices
	contentsDay    time.Time
)

// packValue is a pack's contents value at the latest prices (0 if unknown).
func packValue(id int) float64 {
	contentsMu.RLock()
	defer contentsMu.RUnlock()
	return contentsValue[id]
}

func setContents(packs []evPack, prices map[string]cardPrice, names map[string]string) {
	m := make(map[int]mjBooster, len(packs))
	values := make(map[int]float64, len(packs))
	price := func(uuid string, foil bool) (float64, bool) {
		p := prices[uuid]
		v := p.normal
		if foil {
			v = p.foil
		}
		return v, v > 0
	}
	for _, p := range packs {
		m[p.id] = p.booster
		values[p.id] = packEV(p.booster, price)
	}
	contentsMu.Lock()
	contentsPacks, contentsPrices, contentsNames, contentsValue, contentsDay = m, prices, names, values, time.Now().UTC()
	contentsMu.Unlock()
}

// priceOf is a printing's price in the given finish, falling back to the
// other finish when it has only one.
func priceOf(uuid string, foil bool) float64 {
	p := contentsPrices[uuid]
	a, b := p.normal, p.foil
	if foil {
		a, b = b, a
	}
	if a > 0 {
		return a
	}
	return b
}

// Breakdown is a pack's value by slot and by card.
type Breakdown struct {
	MTGStocksID int     `json:"mtgstocksId"`
	SetName     string  `json:"setName"`
	ProductType string  `json:"productType"`
	Price       float64 `json:"price,omitempty"` // market price; 0 if unknown
	NoSales     bool    `json:"noSales,omitempty"`
	Value       float64 `json:"value"`           // expected value of the contents
	Ratio       float64 `json:"ratio,omitempty"` // price / value
	Premium     bool    `json:"premium"`         // price ≥ premiumRatio × value
	Cards       float64 `json:"cards"`           // cards per pack
	Slots       []Slot  `json:"slots"`
	Top         []Pull  `json:"top"` // the cards that add the most value
	PricesDay   string  `json:"pricesDay"`
}

// Slot is one kind of slot in a pack: how many and what they add up to.
type Slot struct {
	Name  string  `json:"name"`
	Count float64 `json:"count"` // average per pack
	Value float64 `json:"value"`
}

// Pull is one card's share of a pack's value.
type Pull struct {
	Name   string  `json:"name"`
	Foil   bool    `json:"foil"`
	Price  float64 `json:"price"`
	Chance float64 `json:"chance"` // expected copies per pack (≈ chance for rare cards)
	Value  float64 `json:"value"`  // price × chance
}

var camelWord = regexp.MustCompile(`([a-z])([A-Z])`)

// slotName turns MTGJSON sheet names into words: "foilRareMythic" → "foil rare mythic".
func slotName(sheet string) string {
	return strings.ToLower(camelWord.ReplaceAllString(sheet, "$1 $2"))
}

func breakdown(b mjBooster, topN int) (value, cards float64, slots []Slot, top []Pull) {
	slotVal, slotCount := map[string]float64{}, map[string]float64{}
	type key struct {
		uuid string
		foil bool
	}
	pulls := map[key]*Pull{}
	for _, v := range b.Boosters {
		vw := v.Weight / b.BoostersTotalWeight
		for sheet, count := range v.Contents {
			sh, ok := b.Sheets[sheet]
			if !ok || sh.TotalWeight <= 0 {
				continue
			}
			slotCount[sheet] += vw * float64(count)
			cards += vw * float64(count)
			for uuid, w := range sh.Cards {
				chance := vw * float64(count) * w / sh.TotalWeight
				price := priceOf(uuid, sh.Foil)
				slotVal[sheet] += chance * price
				value += chance * price
				k := key{uuid, sh.Foil}
				if pulls[k] == nil {
					name := contentsNames[uuid]
					if name == "" {
						name = "Unnamed card"
					}
					pulls[k] = &Pull{Name: name, Foil: sh.Foil, Price: price}
				}
				pulls[k].Chance += chance
				pulls[k].Value += chance * price
			}
		}
	}
	for sheet, c := range slotCount {
		slots = append(slots, Slot{Name: slotName(sheet), Count: c, Value: slotVal[sheet]})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Value > slots[j].Value })
	for _, p := range pulls {
		if p.Value > 0 {
			top = append(top, *p)
		}
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Value > top[j].Value })
	top = top[:min(len(top), topN)]
	return value, cards, slots, top
}

// Contents breaks a pack's value down (GET /api/packs/{mtgstocksId}/contents).
func (h *TrendsHandler) Contents(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("mtgstocksId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	c, err := getCatalog()
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	var product *SearchResult
	for i := range c.products {
		if c.products[i].MTGStocksID == id {
			product = &c.products[i]
		}
	}
	if product == nil {
		http.Error(w, "not a single pack", http.StatusNotFound)
		return
	}
	contentsMu.RLock()
	defer contentsMu.RUnlock()
	b, ok := contentsPacks[id]
	if !ok {
		msg := "no pull-rate data for this pack (MTGJSON adds sets around release)"
		if len(contentsPacks) == 0 {
			msg = "card prices are still loading after a restart: try again in a few minutes"
		}
		http.Error(w, msg, http.StatusNotFound)
		return
	}
	out := Breakdown{MTGStocksID: id, SetName: product.SetName, ProductType: product.ProductType, NoSales: product.NoSales,
		PricesDay: contentsDay.Format(time.DateOnly)}
	out.Value, out.Cards, out.Slots, out.Top = breakdown(b, 12)
	if product.MarketPrice != nil {
		out.Price = *product.MarketPrice
	}
	if out.Value > 0 && out.Price > 0 {
		out.Ratio = out.Price / out.Value
		out.Premium = out.Ratio >= premiumRatio
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

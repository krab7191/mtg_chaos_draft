package handlers

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Expected value (EV) of a pack's contents: MTGJSON's pull rates (which
// cards each slot can hold, and how often) times TCGplayer's market price of
// each card. Recomputed daily. Backtest (90 days, 2024–2026 sets): a pack
// whose EV rose over the last two weeks tended to rise over the next 3–4
// weeks (rank correlation ~+0.28; ~2% better than a pack whose EV fell). A
// pack priced under its EV predicted little. Shown as information for now.

var (
	// evEnabled: tests turn the daily EV run off (it downloads from MTGJSON).
	evEnabled    = true
	mtgjsonBase  = "https://mtgjson.com/api/v5"
	evFetchDelay = 3 * time.Second // between pull-rate downloads
	evClient     = &http.Client{Timeout: 15 * time.Minute}
)

const (
	evMomentumDays = 14 // EV change is measured over this many days
	evRecheckDays  = 7  // new sets' pull rates are refetched this often
	evNewSetDays   = 60 // ... while released within this many days (or upcoming)
)

type mjSheet struct {
	Cards       map[string]float64 `json:"cards"`
	Foil        bool               `json:"foil"`
	TotalWeight float64            `json:"totalWeight"`
}

type mjBooster struct {
	Boosters []struct {
		Contents map[string]int `json:"contents"`
		Weight   float64        `json:"weight"`
	} `json:"boosters"`
	BoostersTotalWeight float64            `json:"boostersTotalWeight"`
	Sheets              map[string]mjSheet `json:"sheets"`
}

// mjKinds maps MTGJSON booster types to the catalog's pack kinds.
var mjKinds = map[string]string{"play": "play", "collector": "collector", "set": "set", "draft": "draft", "default": "booster"}

// evTargets groups the catalog's single packs by lowercase set code and kind.
func evTargets(c *catalog) map[string]map[string]SearchResult {
	out := map[string]map[string]SearchResult{}
	for _, p := range c.products {
		kind := packKindOf(p.ProductType)
		code := strings.ToLower(strings.TrimSpace(p.SetCode))
		if kind == "" || code == "" || isJumpstart(p.SetName, p.ProductType) {
			continue
		}
		if out[code] == nil {
			out[code] = map[string]SearchResult{}
		}
		out[code][kind] = p
	}
	return out
}

// fetchBoosters downloads a set's pull rates (only the booster types used).
func fetchBoosters(ctx context.Context, code string) (map[string]mjBooster, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s.json.gz", mtgjsonBase, strings.ToUpper(code)), nil)
	if err != nil {
		return nil, err
	}
	resp, err := evClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return map[string]mjBooster{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mtgjson %s: status %d", code, resp.StatusCode)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mtgjson %s: %w", code, err)
	}
	var data struct {
		Data struct {
			Booster map[string]mjBooster `json:"booster"`
		} `json:"data"`
	}
	if err := json.NewDecoder(zr).Decode(&data); err != nil {
		return nil, fmt.Errorf("mtgjson %s: %w", code, err)
	}
	out := map[string]mjBooster{}
	for t, b := range data.Data.Booster {
		if _, ok := mjKinds[t]; ok {
			out[t] = b
		}
	}
	return out, nil
}

// refreshBoosters fetches pull rates for sets that have none stored yet, and
// weekly for new or upcoming sets (MTGJSON adds them around release). A
// failed download stops fetching for today; stored data is still used.
func refreshBoosters(ctx context.Context, pool *pgxpool.Pool, targets map[string]map[string]SearchResult, now time.Time) (map[string]map[string]mjBooster, error) {
	stored, err := db.LoadBoosters(ctx, pool)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(targets))
	for code := range targets {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := map[string]map[string]mjBooster{}
	fetched, stopped := 0, false
	for _, code := range codes {
		s, ok := stored[code]
		var b map[string]mjBooster
		if ok {
			_ = json.Unmarshal(s.Boosters, &b)
		}
		newest := ""
		for _, p := range targets[code] {
			newest = max(newest, p.ReleasedAt)
		}
		recent := newest >= now.AddDate(0, 0, -evNewSetDays).Format(time.DateOnly)
		due := !ok || (now.Sub(s.FetchedAt) >= evRecheckDays*24*time.Hour && (recent || len(b) == 0))
		if due && !stopped {
			if fetched > 0 {
				select {
				case <-time.After(evFetchDelay):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			fresh, err := fetchBoosters(ctx, code)
			if err != nil {
				log.Printf("pack EV: %v (using stored pull rates; retrying tomorrow)", err)
				stopped = true
			} else {
				fetched++
				raw, _ := json.Marshal(fresh)
				if err := db.SaveBoosters(ctx, pool, code, raw); err != nil {
					return nil, err
				}
				b = fresh
			}
		}
		out[code] = b
	}
	if fetched > 0 {
		log.Printf("pack EV: fetched pull rates for %d sets", fetched)
	}
	return out, nil
}

// evPack is one catalog pack and the pull rates that fill it.
type evPack struct {
	id      int
	booster mjBooster
}

// assignBoosters pairs each set's booster types with its catalog packs. Older
// sets' draft boosters are the catalog's plain "Booster Pack" and vice versa.
func assignBoosters(targets map[string]map[string]SearchResult, boosters map[string]map[string]mjBooster) []evPack {
	var out []evPack
	for code, packs := range targets {
		for t, b := range boosters[code] {
			kind := mjKinds[t]
			p, ok := packs[kind]
			if !ok && kind == "draft" {
				p, ok = packs["booster"]
			}
			if !ok && kind == "booster" {
				p, ok = packs["draft"]
			}
			if ok && len(b.Boosters) > 0 && b.BoostersTotalWeight > 0 {
				out = append(out, evPack{id: p.MTGStocksID, booster: b})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// cardPrices are one printing's TCGplayer market prices by day.
type cardPrices struct{ normal, foil, etched map[string]float64 }

// readPrices streams MTGJSON's AllPrices / AllPricesToday JSON and keeps the
// TCGplayer retail prices of the wanted printings.
func readPrices(r io.Reader, want map[string]bool) (map[string]cardPrices, error) {
	dec := json.NewDecoder(bufio.NewReaderSize(r, 1<<20))
	for {
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("prices: no data: %w", err)
		}
		if s, ok := t.(string); ok && s == "data" {
			break
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var entry struct {
		Paper struct {
			TCG struct {
				Retail struct {
					Normal map[string]float64 `json:"normal"`
					Foil   map[string]float64 `json:"foil"`
					Etched map[string]float64 `json:"etched"`
				} `json:"retail"`
			} `json:"tcgplayer"`
		} `json:"paper"`
	}
	out := map[string]cardPrices{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		uuid, _ := t.(string)
		if !want[uuid] {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
			continue
		}
		entry.Paper.TCG.Retail.Normal, entry.Paper.TCG.Retail.Foil, entry.Paper.TCG.Retail.Etched = nil, nil, nil
		if err := dec.Decode(&entry); err != nil {
			return nil, err
		}
		rt := entry.Paper.TCG.Retail
		out[uuid] = cardPrices{normal: rt.Normal, foil: rt.Foil, etched: rt.Etched}
	}
	return out, nil
}

// packEV is a pack's expected value with the given card prices: foil slots
// priced as foil (or etched), others as non-foil, each falling back to the
// other finish when a printing has only one. Unpriced cards count as $0.
func packEV(b mjBooster, price func(uuid string, foil bool) (float64, bool)) float64 {
	total := 0.0
	for _, v := range b.Boosters {
		vw := v.Weight / b.BoostersTotalWeight
		for sheetName, count := range v.Contents {
			sh, ok := b.Sheets[sheetName]
			if !ok || sh.TotalWeight <= 0 {
				continue
			}
			for uuid, w := range sh.Cards {
				p, ok := price(uuid, sh.Foil)
				if !ok {
					p, _ = price(uuid, !sh.Foil)
				}
				total += vw * float64(count) * w / sh.TotalWeight * p
			}
		}
	}
	return total
}

// evByDay computes every pack's EV for each day in the price data, carrying
// each card's last known price forward.
func evByDay(packs []evPack, prices map[string]cardPrices) map[time.Time]map[int]float64 {
	daySet := map[string]bool{}
	for _, cp := range prices {
		for _, m := range []map[string]float64{cp.normal, cp.foil, cp.etched} {
			for d := range m {
				daySet[d] = true
			}
		}
	}
	days := make([]string, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Strings(days)
	type key struct {
		uuid string
		foil bool
	}
	cur := map[key]float64{}
	out := map[time.Time]map[int]float64{}
	for _, d := range days {
		for uuid, cp := range prices {
			if v, ok := cp.normal[d]; ok && v > 0 {
				cur[key{uuid, false}] = v
			}
			if v, ok := cp.foil[d]; ok && v > 0 {
				cur[key{uuid, true}] = v
			} else if v, ok := cp.etched[d]; ok && v > 0 {
				cur[key{uuid, true}] = v
			}
		}
		day, err := time.Parse(time.DateOnly, d)
		if err != nil {
			continue
		}
		evs := map[int]float64{}
		for _, p := range packs {
			ev := packEV(p.booster, func(uuid string, foil bool) (float64, bool) {
				v, ok := cur[key{uuid, foil}]
				return v, ok
			})
			if ev > 0 {
				evs[p.id] = ev
			}
		}
		out[day] = evs
	}
	return out
}

// RefreshEV updates pull rates where due and records today's pack EVs. The
// first run (fewer than two weeks stored) reads MTGJSON's 90-day history
// instead, so EV trends show right away.
func RefreshEV(ctx context.Context, pool *pgxpool.Pool) error {
	c, err := getCatalog()
	if err != nil {
		return err
	}
	targets := evTargets(c)
	boosters, err := refreshBoosters(ctx, pool, targets, time.Now().UTC())
	if err != nil {
		return err
	}
	packs := assignBoosters(targets, boosters)
	want := map[string]bool{}
	for _, p := range packs {
		for _, sh := range p.booster.Sheets {
			for uuid := range sh.Cards {
				want[uuid] = true
			}
		}
	}
	days, err := db.EVDays(ctx, pool)
	if err != nil {
		return err
	}
	file := "AllPricesToday.json.gz"
	if days < evMomentumDays {
		file = "AllPrices.json.gz" // ~150MB once: 90 days of history
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mtgjsonBase+"/"+file, nil)
	if err != nil {
		return err
	}
	resp, err := evClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mtgjson %s: status %d", file, resp.StatusCode)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("mtgjson %s: %w", file, err)
	}
	prices, err := readPrices(zr, want)
	if err != nil {
		return err
	}
	byDay := evByDay(packs, prices)
	if err := db.RecordEV(ctx, pool, byDay); err != nil {
		return err
	}
	log.Printf("pack EV: %d packs over %d days (%s)", len(packs), len(byDay), file)
	return nil
}

// EVInfo is a pack's contents value for the trends tables.
type EVInfo struct {
	Value float64 `json:"value"`
	// Change over the last evMomentumDays (0.06 = up 6%); nil without history.
	Change *float64  `json:"change,omitempty"`
	Day    time.Time `json:"day"`
}

func evInfo(e db.PackEV) *EVInfo {
	info := &EVInfo{Value: e.Value, Day: e.Day}
	if e.Before != nil && *e.Before > 0 {
		ch := e.Value / *e.Before - 1
		info.Change = &ch
	}
	return info
}

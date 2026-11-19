package handlers

import (
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

// CardTrader (https://www.cardtrader.com/en/docs/api/full): a marketplace
// with many foreign-language packs. Its sets carry standard set codes; its
// "Magic Boosters" products (blueprints) are found once per set and saved;
// then each day every booster's listings are read, one request at a time,
// keeping the cheapest listing per language. The API allows 1 request a
// second; a refused request waits a minute and is retried once.

const cardTraderName = "CardTrader"

var (
	cardTraderToken string // CARDTRADER_TOKEN; empty disables CardTrader
	cardTraderBase  = "https://api.cardtrader.com/api/v2"
	cardTraderDelay = 1500 * time.Millisecond
	cardTraderRetry = time.Minute
)

// ctBoosterCategory is CardTrader's "Magic Boosters" category.
const ctBoosterCategory = 5

// ConfigureCardTrader sets the API token (empty turns CardTrader off).
func ConfigureCardTrader(token string) {
	cardTraderToken = strings.TrimSpace(token)
	if cardTraderToken != "" {
		log.Printf("store listings: CardTrader on")
	}
}

// ctLanguages maps CardTrader's mtg_language values to the app's codes.
var ctLanguages = map[string]string{"jp": "ja", "kr": "ko", "zh-CN": "zh", "zh-TW": "zh"}

func ctLanguage(v string) string {
	if v == "" {
		return "en"
	}
	if l, ok := ctLanguages[v]; ok {
		return l
	}
	return v
}

// languageNames for listing titles.
var languageNames = map[string]string{
	"en": "English", "ja": "Japanese", "de": "German", "fr": "French", "it": "Italian", "es": "Spanish",
	"pt": "Portuguese", "zh": "Chinese", "ko": "Korean", "ru": "Russian", "other": "Foreign",
}

// ctClient does one paced request: it waits cardTraderDelay first, and
// once more after a refusal.
type ctClient struct{ started bool }

func (c *ctClient) get(ctx context.Context, path string, out any) error {
	for attempt := 0; ; attempt++ {
		wait := cardTraderDelay
		if !c.started {
			wait, c.started = 0, true
		}
		if attempt > 0 {
			wait = cardTraderRetry
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardTraderBase+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+cardTraderToken)
		req.Header.Set("Accept", "application/json")
		resp, err := storeClient.Do(req)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s %s: status %d", cardTraderName, strings.SplitN(path, "?", 2)[0], resp.StatusCode)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("%s %s: %w", cardTraderName, strings.SplitN(path, "?", 2)[0], err)
		}
		return nil
	}
}

type ctExpansion struct {
	ID     int    `json:"id"`
	GameID int    `json:"game_id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
}

type ctBlueprint struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	CategoryID int    `json:"category_id"`
}

type ctListing struct {
	PriceCents     int    `json:"price_cents"`
	PriceCurrency  string `json:"price_currency"`
	Quantity       int    `json:"quantity"`
	OnVacation     bool   `json:"on_vacation"`
	PropertiesHash struct {
		Language string `json:"mtg_language"`
	} `json:"properties_hash"`
	User struct {
		CountryCode string `json:"country_code"`
	} `json:"user"`
}

// scanCardTrader finds the boosters of every CardTrader set that holds a
// catalog pack and hasn't been scanned yet.
func scanCardTrader(ctx context.Context, pool *pgxpool.Pool, ct *ctClient, c *catalog, idx *packIndex) error {
	var exps []ctExpansion
	if err := ct.get(ctx, "/expansions", &exps); err != nil {
		return err
	}
	// Sets with packs, by set code and by name.
	codes, names := map[string]bool{}, map[string]bool{}
	for _, p := range c.products {
		if packKindOf(p.ProductType) != "" {
			codes[strings.ToLower(p.SetCode)] = true
			names[normTitle(p.SetName)] = true
		}
	}
	scanned, err := db.CardTraderScanned(ctx, pool)
	if err != nil {
		return err
	}
	for _, e := range exps {
		if e.GameID != 1 || scanned[e.ID] || !codes[strings.ToLower(e.Code)] && !names[normTitle(e.Name)] {
			continue
		}
		var bps []ctBlueprint
		if err := ct.get(ctx, fmt.Sprintf("/blueprints/export?expansion_id=%d", e.ID), &bps); err != nil {
			return err
		}
		var found []db.CTBlueprint
		for _, b := range bps {
			if b.CategoryID != ctBoosterCategory {
				continue
			}
			if p, ok := idx.match(b.Name); ok && (strings.EqualFold(p.SetCode, e.Code) || normTitle(p.SetName) == normTitle(e.Name)) {
				found = append(found, db.CTBlueprint{BlueprintID: b.ID, ExpansionID: e.ID, MTGStocksID: p.MTGStocksID, Name: b.Name})
			}
		}
		if err := db.SaveCardTraderExpansion(ctx, pool, e.ID, found); err != nil {
			return err
		}
	}
	return nil
}

// fetchCardTrader returns today's cheapest listing per language for every
// known booster.
func fetchCardTrader(ctx context.Context, pool *pgxpool.Pool, c *catalog, idx *packIndex) ([]db.Offer, int, error) {
	ct := &ctClient{}
	if err := scanCardTrader(ctx, pool, ct, c, idx); err != nil {
		return nil, 0, err
	}
	bps, err := db.CardTraderBlueprints(ctx, pool)
	if err != nil {
		return nil, 0, err
	}
	var out []db.Offer
	for _, b := range bps {
		var byBlueprint map[string][]ctListing
		if err := ct.get(ctx, fmt.Sprintf("/marketplace/products?blueprint_id=%d", b.BlueprintID), &byBlueprint); err != nil {
			return nil, 0, err
		}
		cheapest := map[string]ctListing{}
		for _, l := range byBlueprint[fmt.Sprint(b.BlueprintID)] {
			if l.Quantity <= 0 || l.OnVacation || l.PriceCents <= 0 || l.PriceCurrency != "USD" {
				continue
			}
			lang := ctLanguage(l.PropertiesHash.Language)
			if cur, ok := cheapest[lang]; !ok || l.PriceCents < cur.PriceCents {
				cheapest[lang] = l
			}
		}
		langs := make([]string, 0, len(cheapest))
		for lang := range cheapest {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
		for _, lang := range langs {
			l := cheapest[lang]
			name := languageNames[lang]
			if name == "" {
				name = lang
			}
			out = append(out, db.Offer{
				Store:       cardTraderName,
				URL:         fmt.Sprintf("https://www.cardtrader.com/en/cards/%d#%s", b.BlueprintID, lang),
				MTGStocksID: b.MTGStocksID,
				Title:       fmt.Sprintf("%s · %s · ships from %s", b.Name, name, l.User.CountryCode),
				Price:       float64(l.PriceCents) / 100,
				Available:   true,
				Language:    lang,
			})
		}
	}
	return out, len(bps), nil
}

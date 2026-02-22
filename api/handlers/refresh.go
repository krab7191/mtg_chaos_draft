package handlers

import (
	"context"
	"log"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshPrices fetches current prices from MTGStocks and updates all
// collection packs that have a matching mtgstocks_id.
func RefreshPrices(ctx context.Context, pool *pgxpool.Pool) {
	products, err := getMTGStocksProducts()
	if err != nil {
		log.Printf("price refresh: fetch failed: %v", err)
		return
	}

	prices := make(map[int]float64, len(products))
	for _, p := range products {
		if p.MarketPrice != nil {
			prices[p.MTGStocksID] = *p.MarketPrice
		}
	}

	if err := db.BulkUpdatePrices(ctx, pool, prices); err != nil {
		log.Printf("price refresh: db update failed: %v", err)
		return
	}

	log.Printf("price refresh: updated prices for up to %d products", len(prices))
}

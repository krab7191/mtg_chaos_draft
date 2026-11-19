package handlers

import (
	"context"
	"log"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshPrices applies the current catalog's prices (live, or the saved
// copy when MTGStocks is unreachable) to every pack with an mtgstocks_id.
func RefreshPrices(ctx context.Context, pool *pgxpool.Pool) {
	c, err := getCatalog()
	if err != nil {
		log.Printf("price refresh: fetch failed: %v", err)
		return
	}
	prices := c.prices

	if err := db.BulkUpdatePrices(ctx, pool, prices); err != nil {
		log.Printf("price refresh: db update failed: %v", err)
		return
	}

	log.Printf("price refresh: updated prices for up to %d products (%s catalog from %s)",
		len(prices), c.source, c.fetchedAt.Format("2006-01-02 15:04"))

	recordDailyPrices(ctx, pool, c)
	RefreshStoreOffers(ctx, pool) // before alerts, so today's deals count
	if _, err := NotifyAlerts(ctx, pool); err != nil {
		log.Printf("price alerts: %v", err)
	}
	if err := MaybeRunBacktest(ctx, pool); err != nil {
		log.Printf("backtest: %v", err)
	}
}

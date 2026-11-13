-- The market price at purchase isn't needed: packs are bought at (or below)
-- the market, so only today's market price is compared with what was paid.
ALTER TABLE pack_purchases DROP COLUMN IF EXISTS market_price;

-- What a draft's packs were worth at market, kept apart from what was actually
-- collected from players (proceeds). Players don't always pay market: some
-- don't keep their cards, or the price is estimated on the night.
ALTER TABLE pack_disposals ADD COLUMN IF NOT EXISTS market_value NUMERIC;
UPDATE pack_disposals SET market_value = proceeds WHERE kind = 'draft' AND market_value IS NULL;

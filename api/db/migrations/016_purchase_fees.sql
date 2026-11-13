-- Tax and shipping included in total_price. Market prices don't include them,
-- so deals are judged on the pack price (total_price - fees).
ALTER TABLE pack_purchases ADD COLUMN IF NOT EXISTS fees NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE pack_purchases ADD CONSTRAINT pack_purchases_fees_check CHECK (fees >= 0 AND fees <= total_price);

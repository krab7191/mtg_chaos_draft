-- Store what was paid for the whole purchase, not per pack: a 30-pack display
-- bought for $132.18 doesn't divide evenly, and rounding the per-pack price
-- made the total drift. The per-pack price is derived (total / quantity).
ALTER TABLE pack_purchases ADD COLUMN IF NOT EXISTS total_price NUMERIC;
UPDATE pack_purchases SET total_price = unit_price * quantity WHERE total_price IS NULL;
ALTER TABLE pack_purchases ALTER COLUMN total_price SET NOT NULL;
ALTER TABLE pack_purchases ADD CONSTRAINT pack_purchases_total_price_check CHECK (total_price >= 0);
ALTER TABLE pack_purchases DROP COLUMN IF EXISTS unit_price;

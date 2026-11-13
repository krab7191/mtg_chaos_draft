-- What was actually paid for packs, so purchases can be judged against the
-- market. unit_price is per physical pack. market_price is the market price
-- per pack when the purchase was logged (NULL when unknown, e.g. a purchase
-- logged after the fact).
CREATE TABLE IF NOT EXISTS pack_purchases (
    id           SERIAL PRIMARY KEY,
    pack_id      INTEGER NOT NULL REFERENCES collection_packs(id) ON DELETE CASCADE,
    quantity     INTEGER NOT NULL CHECK (quantity > 0),
    unit_price   NUMERIC NOT NULL CHECK (unit_price >= 0),
    market_price NUMERIC,
    purchased_on DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS pack_purchases_pack_id_idx ON pack_purchases(pack_id);

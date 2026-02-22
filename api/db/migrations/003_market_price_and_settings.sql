ALTER TABLE collection_packs ADD COLUMN IF NOT EXISTS market_price NUMERIC;

CREATE TABLE IF NOT EXISTS weight_settings (
    id                   INTEGER PRIMARY KEY DEFAULT 1,
    price_sensitivity    NUMERIC NOT NULL DEFAULT 0.5,
    scarcity_sensitivity NUMERIC NOT NULL DEFAULT 0.5,
    CONSTRAINT singleton CHECK (id = 1)
);

INSERT INTO weight_settings (id, price_sensitivity, scarcity_sensitivity)
VALUES (1, 0.5, 0.5)
ON CONFLICT (id) DO NOTHING;

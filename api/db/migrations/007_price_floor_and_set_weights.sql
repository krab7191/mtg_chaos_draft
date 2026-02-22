ALTER TABLE weight_settings ADD COLUMN IF NOT EXISTS price_floor NUMERIC NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS set_weight_overrides (
    set_name   TEXT PRIMARY KEY,
    multiplier NUMERIC NOT NULL DEFAULT 1.0
);

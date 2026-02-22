DROP TABLE IF EXISTS set_weight_overrides;

CREATE TABLE IF NOT EXISTS pack_weight_overrides (
    pack_id    INTEGER PRIMARY KEY REFERENCES collection_packs(id) ON DELETE CASCADE,
    multiplier NUMERIC NOT NULL DEFAULT 1.0
);

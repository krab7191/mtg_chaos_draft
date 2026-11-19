-- Expected value of a pack's contents (MTGJSON pull rates x TCGplayer card
-- prices), daily, and each set's pull-rate data it's computed from.
CREATE TABLE IF NOT EXISTS mtgjson_boosters (
    set_code   TEXT        PRIMARY KEY,
    boosters   JSONB       NOT NULL,  -- {"play": {...}, "collector": {...}}; {} when MTGJSON has none yet
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS pack_ev (
    mtgstocks_id INTEGER       NOT NULL,
    day          DATE          NOT NULL,
    ev           NUMERIC(10,2) NOT NULL,
    PRIMARY KEY (mtgstocks_id, day)
);

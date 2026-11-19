-- CardTrader: which of its expansions have been scanned for booster
-- products, and the boosters found (CardTrader blueprint -> MTGStocks pack).
CREATE TABLE IF NOT EXISTS cardtrader_expansions (
    expansion_id INTEGER     PRIMARY KEY,
    scanned_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS cardtrader_blueprints (
    blueprint_id INTEGER PRIMARY KEY,
    expansion_id INTEGER NOT NULL,
    mtgstocks_id INTEGER NOT NULL,
    name         TEXT    NOT NULL
);

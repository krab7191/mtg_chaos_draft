-- Daily market price per MTGStocks sealed product (single packs and booster
-- boxes), recorded daily and backfilled from MTGStocks' price history.
CREATE TABLE IF NOT EXISTS price_history (
    mtgstocks_id INT           NOT NULL,
    day          DATE          NOT NULL,
    price        NUMERIC(10,2) NOT NULL CHECK (price > 0),
    PRIMARY KEY (mtgstocks_id, day)
);

-- Products whose full history has been fetched from MTGStocks.
CREATE TABLE IF NOT EXISTS price_history_backfill (
    mtgstocks_id INT PRIMARY KEY,
    done_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

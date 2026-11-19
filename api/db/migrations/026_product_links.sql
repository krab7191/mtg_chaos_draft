-- Each sealed product's TCGplayer page, from MTGStocks, fetched once.
-- An empty tcg_url means MTGStocks had none (so it isn't asked again).
CREATE TABLE IF NOT EXISTS product_links (
    mtgstocks_id INTEGER     PRIMARY KEY,
    tcg_url      TEXT        NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

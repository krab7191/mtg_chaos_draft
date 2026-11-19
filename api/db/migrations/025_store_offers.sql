-- Booster packs listed at online stores, matched to MTGStocks products and
-- refreshed daily. A store's rows are replaced as a whole after each
-- complete fetch, so a failed fetch keeps the last good listing.
CREATE TABLE IF NOT EXISTS store_offers (
    store        TEXT          NOT NULL,
    url          TEXT          NOT NULL,
    mtgstocks_id INTEGER       NOT NULL,
    title        TEXT          NOT NULL,
    price        NUMERIC(10,2) NOT NULL CHECK (price > 0),
    available    BOOLEAN       NOT NULL,
    seen_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    PRIMARY KEY (store, url)
);
CREATE INDEX IF NOT EXISTS store_offers_product ON store_offers (mtgstocks_id);

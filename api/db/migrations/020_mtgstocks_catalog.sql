-- Last good copy of the MTGStocks sealed catalog (raw /sealed response), so
-- search and pricing keep working when MTGStocks refuses our requests.
CREATE TABLE IF NOT EXISTS mtgstocks_catalog (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    body       BYTEA       NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    source     TEXT        NOT NULL
);

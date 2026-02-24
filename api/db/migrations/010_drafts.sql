CREATE TABLE IF NOT EXISTS drafts (
    id          SERIAL PRIMARY KEY,
    drafted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    approved_by INTEGER REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS draft_picks (
    id           SERIAL PRIMARY KEY,
    draft_id     INTEGER NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
    pack_id      INTEGER REFERENCES collection_packs(id) ON DELETE SET NULL,
    set_name     TEXT NOT NULL,
    product_type TEXT NOT NULL,
    market_price NUMERIC
);

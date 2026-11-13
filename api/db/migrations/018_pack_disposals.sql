-- Packs that left the collection, and what came in for them: opened in an
-- approved draft (players pay the market price of each pick) or sold.
-- Together with pack_purchases this gives realized and unrealized gains.
CREATE TABLE IF NOT EXISTS pack_disposals (
    id          SERIAL PRIMARY KEY,
    pack_id     INTEGER NOT NULL REFERENCES collection_packs(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('draft', 'sale')),
    -- Kept when the draft is deleted: its packs were still opened.
    draft_id    INTEGER REFERENCES drafts(id) ON DELETE SET NULL,
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    proceeds    NUMERIC NOT NULL CHECK (proceeds >= 0),
    fees        NUMERIC NOT NULL DEFAULT 0 CHECK (fees >= 0 AND fees <= proceeds),
    disposed_on DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS pack_disposals_pack_id_idx ON pack_disposals(pack_id);

-- Drafts approved before this table existed.
INSERT INTO pack_disposals (pack_id, kind, draft_id, quantity, proceeds, disposed_on)
SELECT dp.pack_id, 'draft', d.id,
       COUNT(*)::int * CASE WHEN cp.cards_per_pack <= 5 THEN 3 WHEN cp.cards_per_pack <= 8 THEN 2 ELSE 1 END,
       COALESCE(SUM(dp.market_price), 0),
       d.drafted_at::date
FROM drafts d
JOIN draft_picks dp ON dp.draft_id = d.id
JOIN collection_packs cp ON cp.id = dp.pack_id
WHERE d.approved_at IS NOT NULL
GROUP BY d.id, dp.pack_id, cp.cards_per_pack, d.drafted_at;

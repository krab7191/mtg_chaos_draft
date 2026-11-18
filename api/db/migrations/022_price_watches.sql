-- Packs and boxes being watched on the trends page. drop_alert, when set, is
-- the fraction below the release price that should raise an alert.
CREATE TABLE IF NOT EXISTS price_watches (
    mtgstocks_id INT PRIMARY KEY,
    drop_alert   NUMERIC(4,3) CHECK (drop_alert IS NULL OR (drop_alert > 0 AND drop_alert < 1)),
    added_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

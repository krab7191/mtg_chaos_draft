-- Alert reasons already sent to the phone for each watch, so a product
-- sitting at its floor notifies once, and again only after the alert clears.
ALTER TABLE price_watches ADD COLUMN IF NOT EXISTS notified TEXT[] NOT NULL DEFAULT '{}';

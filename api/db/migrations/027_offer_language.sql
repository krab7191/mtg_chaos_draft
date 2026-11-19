-- Store listings keep foreign-language packs (fine for chaos drafts), labelled.
ALTER TABLE store_offers ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT 'en';

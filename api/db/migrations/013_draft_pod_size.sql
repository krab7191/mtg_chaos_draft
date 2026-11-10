-- Pod size (number of players) for a draft. Each player opens 3 packs, so a
-- draft has pod_size * 3 picks. Existing drafts were fixed at 12 picks (4 players).
ALTER TABLE drafts ADD COLUMN IF NOT EXISTS pod_size INTEGER NOT NULL DEFAULT 4;

-- Card names for each set's pull-rate data, for the pack contents breakdown.
ALTER TABLE mtgjson_boosters ADD COLUMN IF NOT EXISTS names JSONB NOT NULL DEFAULT '{}';

-- Small app-wide settings by key (e.g. which pack languages deal alerts cover).
CREATE TABLE IF NOT EXISTS app_settings (
    key        TEXT        PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

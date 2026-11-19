-- Floor-signal rules per product kind ('regular', 'collector'): the active
-- one, proposals from the quarterly backtest, and past ones.
CREATE TABLE IF NOT EXISTS floor_rules (
    id         SERIAL PRIMARY KEY,
    kind       TEXT        NOT NULL,
    rule       JSONB       NOT NULL,
    status     TEXT        NOT NULL CHECK (status IN ('active', 'proposed', 'dismissed', 'retired')),
    -- Cross-validated results: {"current": {...}, "proposed": {...}, "products": n}
    metrics    JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS floor_rules_one_active ON floor_rules (kind) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS floor_rules_one_proposed ON floor_rules (kind) WHERE status = 'proposed';

-- When the backtest last ran, and what it found.
CREATE TABLE IF NOT EXISTS backtest_runs (
    id      SERIAL PRIMARY KEY,
    ran_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    summary JSONB       NOT NULL
);

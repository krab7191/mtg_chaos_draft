package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoredRule is a floor rule row. Rule and Metrics are JSON owned by the
// handlers package.
type StoredRule struct {
	ID        int             `json:"id"`
	Kind      string          `json:"kind"`
	Rule      json.RawMessage `json:"rule"`
	Status    string          `json:"status"`
	Metrics   json.RawMessage `json:"metrics"`
	CreatedAt time.Time       `json:"createdAt"`
}

// ErrNoRule means there's no such rule in the expected state.
var ErrNoRule = errors.New("no such rule")

func scanRules(rows pgx.Rows) ([]StoredRule, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (StoredRule, error) {
		var s StoredRule
		err := r.Scan(&s.ID, &s.Kind, &s.Rule, &s.Status, &s.Metrics, &s.CreatedAt)
		return s, err
	})
}

// RulesByStatus lists rules in a status ('active' or 'proposed').
func RulesByStatus(ctx context.Context, pool *pgxpool.Pool, status string) ([]StoredRule, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, kind, rule, status, metrics, created_at FROM floor_rules
		WHERE status = $1 ORDER BY kind`, status)
	if err != nil {
		return nil, err
	}
	return scanRules(rows)
}

// SetActiveRule makes rule the active one for kind, retiring the previous.
func SetActiveRule(ctx context.Context, pool *pgxpool.Pool, kind string, rule json.RawMessage) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE floor_rules SET status = 'retired', decided_at = NOW() WHERE kind = $1 AND status = 'active'`, kind); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO floor_rules (kind, rule, status, decided_at) VALUES ($1, $2, 'active', NOW())`, kind, rule); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ProposeRule records a backtest proposal for kind, replacing any open one.
func ProposeRule(ctx context.Context, pool *pgxpool.Pool, kind string, rule, metrics json.RawMessage) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE floor_rules SET status = 'dismissed', decided_at = NOW() WHERE kind = $1 AND status = 'proposed'`, kind); err != nil {
		return 0, err
	}
	var id int
	if err := tx.QueryRow(ctx, `INSERT INTO floor_rules (kind, rule, status, metrics) VALUES ($1, $2, 'proposed', $3) RETURNING id`,
		kind, rule, metrics).Scan(&id); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// DecideProposal applies (makes active, retiring the old) or dismisses a
// proposal. Returns the decided rule, or ErrNoRule if it isn't an open one.
func DecideProposal(ctx context.Context, pool *pgxpool.Pool, id int, apply bool) (*StoredRule, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id, kind, rule, status, metrics, created_at FROM floor_rules
		WHERE id = $1 AND status = 'proposed' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	found, err := scanRules(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrNoRule
	}
	r := found[0]
	if apply {
		if _, err := tx.Exec(ctx, `UPDATE floor_rules SET status = 'retired', decided_at = NOW() WHERE kind = $1 AND status = 'active'`, r.Kind); err != nil {
			return nil, err
		}
		r.Status = "active"
	} else {
		r.Status = "dismissed"
	}
	if _, err := tx.Exec(ctx, `UPDATE floor_rules SET status = $2, decided_at = NOW() WHERE id = $1`, id, r.Status); err != nil {
		return nil, err
	}
	return &r, tx.Commit(ctx)
}

// RecordBacktest stores a backtest run's summary.
func RecordBacktest(ctx context.Context, pool *pgxpool.Pool, summary json.RawMessage) error {
	_, err := pool.Exec(ctx, `INSERT INTO backtest_runs (summary) VALUES ($1)`, summary)
	return err
}

// LastBacktest returns when the backtest last ran and its summary, or a zero
// time if it never has.
func LastBacktest(ctx context.Context, pool *pgxpool.Pool) (time.Time, json.RawMessage, error) {
	var at time.Time
	var summary json.RawMessage
	err := pool.QueryRow(ctx, `SELECT ran_at, summary FROM backtest_runs ORDER BY ran_at DESC LIMIT 1`).Scan(&at, &summary)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil, nil
	}
	return at, summary, err
}

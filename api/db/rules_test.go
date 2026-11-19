package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestFloorRuleLifecycle(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "floor_rules", "backtest_runs")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE floor_rules, backtest_runs`); err != nil {
		t.Fatal(err)
	}
	ruleA, ruleB := json.RawMessage(`{"minAge":150}`), json.RawMessage(`{"minAge":120}`)

	if err := db.SetActiveRule(ctx, pool, "regular", ruleA); err != nil {
		t.Fatal(err)
	}
	// A second proposal for the same kind replaces the first.
	first, err := db.ProposeRule(ctx, pool, "regular", ruleB, json.RawMessage(`{"n":1}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.ProposeRule(ctx, pool, "regular", ruleB, json.RawMessage(`{"n":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := db.RulesByStatus(ctx, pool, "proposed"); len(p) != 1 || p[0].ID != second {
		t.Fatalf("open proposals: %+v", p)
	}
	if _, err := db.DecideProposal(ctx, pool, first, true); !errors.Is(err, db.ErrNoRule) {
		t.Errorf("a replaced proposal can't be applied: %v", err)
	}

	applied, err := db.DecideProposal(ctx, pool, second, true)
	if err != nil || applied.Status != "active" || applied.Kind != "regular" {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	active, _ := db.RulesByStatus(ctx, pool, "active")
	if len(active) != 1 || string(active[0].Rule) != `{"minAge": 120}` {
		t.Errorf("active after apply: %+v", active)
	}

	third, _ := db.ProposeRule(ctx, pool, "collector", ruleA, nil)
	if d, err := db.DecideProposal(ctx, pool, third, false); err != nil || d.Status != "dismissed" {
		t.Errorf("dismiss: %+v %v", d, err)
	}
	if p, _ := db.RulesByStatus(ctx, pool, "proposed"); len(p) != 0 {
		t.Errorf("nothing should be open: %+v", p)
	}

	at, _, err := db.LastBacktest(ctx, pool)
	if err != nil || !at.IsZero() {
		t.Errorf("never run: %v %v", at, err)
	}
	if err := db.RecordBacktest(ctx, pool, json.RawMessage(`{"products":163}`)); err != nil {
		t.Fatal(err)
	}
	at, summary, err := db.LastBacktest(ctx, pool)
	if err != nil || at.IsZero() || string(summary) != `{"products": 163}` {
		t.Errorf("last run: %v %s %v", at, summary, err)
	}
}

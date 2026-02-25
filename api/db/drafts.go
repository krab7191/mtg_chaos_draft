package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDraftNotFound = errors.New("draft not found or already approved")

// InsufficientQuantityError is returned when approving a draft would push one
// or more packs below zero quantity.
type InsufficientQuantityError struct {
	PackNames []string
}

func (e *InsufficientQuantityError) Error() string {
	return fmt.Sprintf("insufficient stock for: %s", strings.Join(e.PackNames, ", "))
}

type DraftPick struct {
	ID          int      `json:"id"`
	PackID      *int     `json:"packId"`
	SetName     string   `json:"setName"`
	ProductType string   `json:"productType"`
	MarketPrice *float64 `json:"marketPrice"`
}

type Draft struct {
	ID         int         `json:"id"`
	DraftedAt  time.Time   `json:"draftedAt"`
	ApprovedAt *time.Time  `json:"approvedAt"`
	ApprovedBy *int        `json:"approvedBy"`
	Picks      []DraftPick `json:"picks"`
}

func DeleteDraft(ctx context.Context, pool *pgxpool.Pool, draftID int) error {
	_, err := pool.Exec(ctx, `DELETE FROM drafts WHERE id = $1`, draftID)
	return err
}

func CreateDraft(ctx context.Context, pool *pgxpool.Pool, picks []DraftPick) (*Draft, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var draft Draft
	err = tx.QueryRow(ctx,
		`INSERT INTO drafts DEFAULT VALUES RETURNING id, drafted_at, approved_at, approved_by`,
	).Scan(&draft.ID, &draft.DraftedAt, &draft.ApprovedAt, &draft.ApprovedBy)
	if err != nil {
		return nil, err
	}

	draft.Picks = make([]DraftPick, 0, len(picks))
	for _, p := range picks {
		var pick DraftPick
		err = tx.QueryRow(ctx,
			`INSERT INTO draft_picks (draft_id, pack_id, set_name, product_type, market_price)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, pack_id, set_name, product_type, market_price`,
			draft.ID, p.PackID, p.SetName, p.ProductType, p.MarketPrice,
		).Scan(&pick.ID, &pick.PackID, &pick.SetName, &pick.ProductType, &pick.MarketPrice)
		if err != nil {
			return nil, err
		}
		draft.Picks = append(draft.Picks, pick)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &draft, nil
}

func ListDrafts(ctx context.Context, pool *pgxpool.Pool) ([]Draft, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, drafted_at, approved_at, approved_by FROM drafts ORDER BY drafted_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []Draft
	draftIndex := map[int]int{}
	for rows.Next() {
		var d Draft
		if err := rows.Scan(&d.ID, &d.DraftedAt, &d.ApprovedAt, &d.ApprovedBy); err != nil {
			return nil, err
		}
		d.Picks = []DraftPick{}
		draftIndex[d.ID] = len(drafts)
		drafts = append(drafts, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(drafts) == 0 {
		return []Draft{}, nil
	}

	ids := make([]int, len(drafts))
	for i, d := range drafts {
		ids[i] = d.ID
	}
	pickRows, err := pool.Query(ctx,
		`SELECT id, draft_id, pack_id, set_name, product_type, market_price
		 FROM draft_picks WHERE draft_id = ANY($1) ORDER BY id`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer pickRows.Close()

	for pickRows.Next() {
		var p DraftPick
		var draftID int
		if err := pickRows.Scan(&p.ID, &draftID, &p.PackID, &p.SetName, &p.ProductType, &p.MarketPrice); err != nil {
			return nil, err
		}
		idx := draftIndex[draftID]
		drafts[idx].Picks = append(drafts[idx].Picks, p)
	}
	if err := pickRows.Err(); err != nil {
		return nil, err
	}

	return drafts, nil
}

func ApproveDraft(ctx context.Context, pool *pgxpool.Pool, draftID, userID int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx,
		`UPDATE drafts SET approved_at = NOW(), approved_by = $2 WHERE id = $1 AND approved_at IS NULL`,
		draftID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDraftNotFound
	}

	// Check whether any pack would go below zero. Group by pack so that a pack
	// picked multiple times in one draft is counted correctly.
	shortRows, err := tx.Query(ctx, `
		SELECT cp.set_name
		FROM draft_picks dp
		JOIN collection_packs cp ON cp.id = dp.pack_id
		WHERE dp.draft_id = $1 AND dp.pack_id IS NOT NULL
		GROUP BY cp.id, cp.set_name, cp.quantity, cp.cards_per_pack
		HAVING cp.quantity < COUNT(*) * CASE WHEN cp.cards_per_pack <= 5 THEN 3 WHEN cp.cards_per_pack <= 8 THEN 2 ELSE 1 END
	`, draftID)
	if err != nil {
		return err
	}
	defer shortRows.Close()

	var short []string
	for shortRows.Next() {
		var name string
		if err := shortRows.Scan(&name); err != nil {
			return err
		}
		short = append(short, name)
	}
	if err := shortRows.Err(); err != nil {
		return err
	}
	if len(short) > 0 {
		return &InsufficientQuantityError{PackNames: short}
	}

	// Decrement each pack by the number of physical packs consumed (slots × packs-per-slot).
	_, err = tx.Exec(ctx, `
		UPDATE collection_packs
		SET quantity = quantity - sub.cnt * CASE WHEN collection_packs.cards_per_pack <= 5 THEN 3 WHEN collection_packs.cards_per_pack <= 8 THEN 2 ELSE 1 END
		FROM (
			SELECT pack_id, COUNT(*)::int AS cnt
			FROM draft_picks
			WHERE draft_id = $1 AND pack_id IS NOT NULL
			GROUP BY pack_id
		) sub
		WHERE collection_packs.id = sub.pack_id
	`, draftID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

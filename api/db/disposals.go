package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Disposal is packs leaving the collection: opened in a draft or sold.
type Disposal struct {
	ID          int       `json:"id"`
	PackID      int       `json:"packId"`
	SetName     string    `json:"setName"`
	ProductType string    `json:"productType"`
	Kind        string    `json:"kind"` // "draft" or "sale"
	DraftID     *int      `json:"draftId"`
	Quantity    int       `json:"quantity"`
	Proceeds    float64   `json:"proceeds"`    // money in for all packs, before fees
	MarketValue *float64  `json:"marketValue"` // drafts: what the packs were worth at market
	Fees        float64   `json:"fees"`        // selling fees & shipping paid out of proceeds
	DisposedOn  string    `json:"disposedOn"`
	CreatedAt   time.Time `json:"createdAt"`
}

var (
	// ErrDraftNotRecorded is returned when a draft has no recorded packs (not approved).
	ErrDraftNotRecorded = errors.New("draft has no recorded packs")
	// ErrSaleNotFound is returned when a sale to change doesn't exist.
	ErrSaleNotFound = errors.New("sale not found")
	// ErrNotEnoughStock is returned when selling more packs than are in stock.
	ErrNotEnoughStock = errors.New("not enough packs in stock")
)

const disposalSelect = `
	SELECT pd.id, pd.pack_id, cp.set_name, cp.product_type, pd.kind, pd.draft_id,
	       pd.quantity, pd.proceeds, pd.market_value, pd.fees, to_char(pd.disposed_on, 'YYYY-MM-DD'), pd.created_at`

func scanDisposal(row interface{ Scan(...any) error }) (*Disposal, error) {
	d := &Disposal{}
	err := row.Scan(&d.ID, &d.PackID, &d.SetName, &d.ProductType, &d.Kind, &d.DraftID,
		&d.Quantity, &d.Proceeds, &d.MarketValue, &d.Fees, &d.DisposedOn, &d.CreatedAt)
	return d, err
}

// recordDraftDisposals logs the packs an approved draft opened: physical packs
// per pack (slots × packs per slot) and their market value. What was collected
// starts out as the market value; SetDraftCollected corrects it.
func recordDraftDisposals(ctx context.Context, tx pgx.Tx, draftID int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO pack_disposals (pack_id, kind, draft_id, quantity, proceeds, market_value, disposed_on)
		SELECT dp.pack_id, 'draft', d.id,
		       COUNT(*)::int * CASE WHEN cp.cards_per_pack <= 5 THEN 3 WHEN cp.cards_per_pack <= 8 THEN 2 ELSE 1 END,
		       COALESCE(SUM(dp.market_price), 0),
		       COALESCE(SUM(dp.market_price), 0),
		       d.drafted_at::date
		FROM drafts d
		JOIN draft_picks dp ON dp.draft_id = d.id
		JOIN collection_packs cp ON cp.id = dp.pack_id
		WHERE d.id = $1
		GROUP BY d.id, dp.pack_id, cp.cards_per_pack, d.drafted_at`, draftID)
	return err
}

// takeStock removes n packs from a pack's stock, failing when there aren't enough.
func takeStock(ctx context.Context, tx pgx.Tx, packID, n int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE collection_packs SET quantity = quantity - $2 WHERE id = $1 AND quantity >= $2`, packID, n)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM collection_packs WHERE id = $1)`, packID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrPackNotFound
	}
	return ErrNotEnoughStock
}

func returnStock(ctx context.Context, tx pgx.Tx, packID, n int) error {
	_, err := tx.Exec(ctx, `UPDATE collection_packs SET quantity = quantity + $2 WHERE id = $1`, packID, n)
	return err
}

func getDisposal(ctx context.Context, q Querier, id int) (*Disposal, error) {
	return scanDisposal(q.QueryRow(ctx, disposalSelect+`
		FROM pack_disposals pd JOIN collection_packs cp ON cp.id = pd.pack_id
		WHERE pd.id = $1`, id))
}

// RecordSale takes sold packs out of stock and logs what they sold for.
// An empty soldOn means today.
func RecordSale(ctx context.Context, pool *pgxpool.Pool, packID, quantity int, proceeds, fees float64, soldOn string) (*Disposal, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := takeStock(ctx, tx, packID, quantity); err != nil {
		return nil, err
	}
	var id int
	if err := tx.QueryRow(ctx, `
		INSERT INTO pack_disposals (pack_id, kind, quantity, proceeds, fees, disposed_on)
		VALUES ($1, 'sale', $2, $3, $4, COALESCE(NULLIF($5, '')::date, CURRENT_DATE))
		RETURNING id`, packID, quantity, proceeds, fees, soldOn).Scan(&id); err != nil {
		return nil, err
	}
	d, err := getDisposal(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit(ctx)
}

// UpdateSale corrects a sale. A change in quantity moves the difference in or
// out of stock.
func UpdateSale(ctx context.Context, pool *pgxpool.Pool, id, quantity int, proceeds, fees float64, soldOn string) (*Disposal, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var packID, oldQty int
	err = tx.QueryRow(ctx,
		`SELECT pack_id, quantity FROM pack_disposals WHERE id = $1 AND kind = 'sale' FOR UPDATE`, id,
	).Scan(&packID, &oldQty)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSaleNotFound
	}
	if err != nil {
		return nil, err
	}
	switch delta := quantity - oldQty; {
	case delta > 0:
		if err := takeStock(ctx, tx, packID, delta); err != nil {
			return nil, err
		}
	case delta < 0:
		if err := returnStock(ctx, tx, packID, -delta); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE pack_disposals SET quantity = $2, proceeds = $3, fees = $4, disposed_on = $5::date
		WHERE id = $1`, id, quantity, proceeds, fees, soldOn); err != nil {
		return nil, err
	}
	d, err := getDisposal(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return d, tx.Commit(ctx)
}

// DeleteSale undoes a sale: the packs go back into stock.
func DeleteSale(ctx context.Context, pool *pgxpool.Pool, id int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var packID, qty int
	err = tx.QueryRow(ctx,
		`DELETE FROM pack_disposals WHERE id = $1 AND kind = 'sale' RETURNING pack_id, quantity`, id,
	).Scan(&packID, &qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSaleNotFound
	}
	if err != nil {
		return err
	}
	if err := returnStock(ctx, tx, packID, qty); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetDraftCollected records what was actually collected from players for a
// draft. It's split across the draft's packs by market value (by pack count if
// no market values are known), and the draft's records are returned.
func SetDraftCollected(ctx context.Context, pool *pgxpool.Pool, draftID int, amount float64) ([]Disposal, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE pack_disposals pd SET proceeds = CASE
			WHEN t.market > 0 THEN $2 * COALESCE(pd.market_value, 0) / t.market
			ELSE $2 * pd.quantity / t.packs
		END
		FROM (
			SELECT SUM(COALESCE(market_value, 0)) AS market, SUM(quantity)::numeric AS packs
			FROM pack_disposals WHERE draft_id = $1 AND kind = 'draft'
		) t
		WHERE pd.draft_id = $1 AND pd.kind = 'draft'`, draftID, amount)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrDraftNotRecorded
	}
	rows, err := tx.Query(ctx, disposalSelect+`
		FROM pack_disposals pd JOIN collection_packs cp ON cp.id = pd.pack_id
		WHERE pd.draft_id = $1 AND pd.kind = 'draft'
		ORDER BY pd.id`, draftID)
	if err != nil {
		return nil, err
	}
	var out []Disposal
	for rows.Next() {
		d, err := scanDisposal(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ListDisposals returns every draft and sale, newest first.
func ListDisposals(ctx context.Context, pool *pgxpool.Pool) ([]Disposal, error) {
	rows, err := pool.Query(ctx, disposalSelect+`
		FROM pack_disposals pd JOIN collection_packs cp ON cp.id = pd.pack_id
		ORDER BY pd.disposed_on DESC, pd.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Disposal
	for rows.Next() {
		d, err := scanDisposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

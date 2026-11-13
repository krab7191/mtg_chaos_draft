package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Purchase is one buy of a pack, with the pack's details for display.
type Purchase struct {
	ID           int       `json:"id"`
	PackID       int       `json:"packId"`
	SetName      string    `json:"setName"`
	ProductType  string    `json:"productType"`
	Quantity     int       `json:"quantity"`
	TotalPrice   float64   `json:"totalPrice"`   // paid for the whole purchase, fees included
	Fees         float64   `json:"fees"`         // tax & shipping within TotalPrice
	UnitPrice    float64   `json:"unitPrice"`    // totalPrice / quantity
	CurrentPrice *float64  `json:"currentPrice"` // market per pack now
	PurchasedOn  string    `json:"purchasedOn"`  // YYYY-MM-DD
	CreatedAt    time.Time `json:"createdAt"`
}

// ErrPackNotFound is returned when a purchase references a pack that doesn't exist.
var ErrPackNotFound = errors.New("pack not found")

// ErrPurchaseNotFound is returned when updating a purchase that doesn't exist.
var ErrPurchaseNotFound = errors.New("purchase not found")

const purchaseSelect = `
	SELECT pp.id, pp.pack_id, cp.set_name, cp.product_type, pp.quantity,
	       pp.total_price, pp.fees, pp.total_price / pp.quantity, cp.market_price,
	       to_char(pp.purchased_on, 'YYYY-MM-DD'), pp.created_at`

func scanPurchase(row interface{ Scan(...any) error }) (*Purchase, error) {
	p := &Purchase{}
	err := row.Scan(&p.ID, &p.PackID, &p.SetName, &p.ProductType, &p.Quantity,
		&p.TotalPrice, &p.Fees, &p.UnitPrice, &p.CurrentPrice, &p.PurchasedOn, &p.CreatedAt)
	return p, err
}

// AddPurchase records a purchase. totalPrice is what was paid for all of
// quantity packs, including fees (tax & shipping). An empty purchasedOn means today.
func AddPurchase(ctx context.Context, q Querier, packID, quantity int, totalPrice, fees float64, purchasedOn string) (*Purchase, error) {
	p, err := scanPurchase(q.QueryRow(ctx, `
		WITH pp AS (
			INSERT INTO pack_purchases (pack_id, quantity, total_price, fees, purchased_on)
			VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, '')::date, CURRENT_DATE))
			RETURNING *
		)`+purchaseSelect+`
		FROM pp JOIN collection_packs cp ON cp.id = pp.pack_id`,
		packID, quantity, totalPrice, fees, purchasedOn))
	if isForeignKeyViolation(err) {
		return nil, ErrPackNotFound
	}
	return p, err
}

// AddPackWithPurchase adds packs to the collection (merging stock like AddPack)
// and records what was paid for them, atomically.
func AddPackWithPurchase(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int, name, setName, setCode, productType string, quantity int, weight float64, marketPrice *float64, cardsPerPack int, totalPrice, fees float64, purchasedOn string) (*CollectionPack, *Purchase, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pack, err := AddPack(ctx, tx, mtgstocksID, name, setName, setCode, productType, quantity, weight, marketPrice, cardsPerPack)
	if err != nil {
		return nil, nil, err
	}
	purchase, err := AddPurchase(ctx, tx, pack.ID, quantity, totalPrice, fees, purchasedOn)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return pack, purchase, nil
}

// LogPurchase records a purchase of a pack already in the collection. With
// addToStock the bought packs are also added to its quantity; without it the
// purchase is only logged (e.g. back-filling packs that are already counted).
func LogPurchase(ctx context.Context, pool *pgxpool.Pool, packID, quantity int, totalPrice, fees float64, purchasedOn string, addToStock bool) (*Purchase, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if addToStock {
		tag, err := tx.Exec(ctx, `UPDATE collection_packs SET quantity = quantity + $2 WHERE id = $1`, packID, quantity)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrPackNotFound
		}
	}
	purchase, err := AddPurchase(ctx, tx, packID, quantity, totalPrice, fees, purchasedOn)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return purchase, nil
}

// ListPurchases returns every purchase, newest first.
func ListPurchases(ctx context.Context, pool *pgxpool.Pool) ([]Purchase, error) {
	rows, err := pool.Query(ctx, purchaseSelect+`
		FROM pack_purchases pp JOIN collection_packs cp ON cp.id = pp.pack_id
		ORDER BY pp.purchased_on DESC, pp.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Purchase
	for rows.Next() {
		p, err := scanPurchase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdatePurchase corrects a purchase's quantity, total paid, fees and date.
// Stock is left alone, as with DeletePurchase.
func UpdatePurchase(ctx context.Context, pool *pgxpool.Pool, id, quantity int, totalPrice, fees float64, purchasedOn string) (*Purchase, error) {
	p, err := scanPurchase(pool.QueryRow(ctx, `
		WITH pp AS (
			UPDATE pack_purchases SET
				quantity     = $2,
				total_price  = $3,
				fees         = $4,
				purchased_on = $5::date
			WHERE id = $1
			RETURNING *
		)`+purchaseSelect+`
		FROM pp JOIN collection_packs cp ON cp.id = pp.pack_id`,
		id, quantity, totalPrice, fees, purchasedOn))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPurchaseNotFound
	}
	return p, err
}

// DeletePurchase removes a purchase record. Stock is left alone: the packs may
// already have been opened.
func DeletePurchase(ctx context.Context, pool *pgxpool.Pool, id int) error {
	_, err := pool.Exec(ctx, `DELETE FROM pack_purchases WHERE id = $1`, id)
	return err
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CollectionPack struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	SetName     string    `json:"setName"`
	ProductType string    `json:"productType"`
	MTGStocksID *int      `json:"mtgstocksId"`
	MarketPrice *float64  `json:"marketPrice"`
	Quantity    int       `json:"quantity"`
	Weight      float64   `json:"weight"`
	Notes       string    `json:"notes"`
	AddedAt     time.Time `json:"addedAt"`
}

const packColumns = `id, name, set_name, product_type, mtgstocks_id,
	market_price, quantity, weight, COALESCE(notes, ''), added_at`

func scanPack(row interface{ Scan(...any) error }) (*CollectionPack, error) {
	p := &CollectionPack{}
	err := row.Scan(&p.ID, &p.Name, &p.SetName, &p.ProductType,
		&p.MTGStocksID, &p.MarketPrice, &p.Quantity, &p.Weight, &p.Notes, &p.AddedAt)
	return p, err
}

func ListCollection(ctx context.Context, pool *pgxpool.Pool) ([]CollectionPack, error) {
	rows, err := pool.Query(ctx, `SELECT `+packColumns+` FROM collection_packs ORDER BY set_name, product_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []CollectionPack
	for rows.Next() {
		p, err := scanPack(rows)
		if err != nil {
			return nil, err
		}
		packs = append(packs, *p)
	}
	return packs, rows.Err()
}

func AddPack(ctx context.Context, pool *pgxpool.Pool, mtgstocksID int, name, setName, productType string, quantity int, weight float64, marketPrice *float64) (*CollectionPack, error) {
	return scanPack(pool.QueryRow(ctx, `
		INSERT INTO collection_packs (name, set_name, product_type, mtgstocks_id, quantity, weight, market_price)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (mtgstocks_id) DO UPDATE SET quantity = collection_packs.quantity + EXCLUDED.quantity
		RETURNING `+packColumns,
		name, setName, productType, mtgstocksID, quantity, weight, marketPrice))
}

func UpdatePack(ctx context.Context, pool *pgxpool.Pool, id int, quantity *int, weight *float64, notes *string, mtgstocksID *int, marketPrice *float64) (*CollectionPack, error) {
	return scanPack(pool.QueryRow(ctx, `
		UPDATE collection_packs SET
			quantity     = COALESCE($2, quantity),
			weight       = COALESCE($3, weight),
			notes        = COALESCE($4, notes),
			mtgstocks_id = COALESCE($5, mtgstocks_id),
			market_price = COALESCE($6, market_price)
		WHERE id = $1
		RETURNING `+packColumns,
		id, quantity, weight, notes, mtgstocksID, marketPrice))
}

func DeletePack(ctx context.Context, pool *pgxpool.Pool, id int) error {
	_, err := pool.Exec(ctx, `DELETE FROM collection_packs WHERE id = $1`, id)
	return err
}

func BulkUpdatePrices(ctx context.Context, pool *pgxpool.Pool, prices map[int]float64) error {
	if len(prices) == 0 {
		return nil
	}
	ids := make([]int, 0, len(prices))
	vals := make([]float64, 0, len(prices))
	for id, price := range prices {
		ids = append(ids, id)
		vals = append(vals, price)
	}
	_, err := pool.Exec(ctx, `
		UPDATE collection_packs
		SET market_price = v.price
		FROM unnest($1::int[], $2::numeric[]) AS v(mtgstocks_id, price)
		WHERE collection_packs.mtgstocks_id = v.mtgstocks_id
	`, ids, vals)
	return err
}

func GetPacksByIDs(ctx context.Context, pool *pgxpool.Pool, ids []int) ([]CollectionPack, error) {
	rows, err := pool.Query(ctx, `SELECT `+packColumns+` FROM collection_packs WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []CollectionPack
	for rows.Next() {
		p, err := scanPack(rows)
		if err != nil {
			return nil, err
		}
		packs = append(packs, *p)
	}
	return packs, rows.Err()
}

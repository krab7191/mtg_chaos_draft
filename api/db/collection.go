package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CollectionPack struct {
	ID              int       `json:"id"`
	ScryfallSetCode string    `json:"scryfallSetCode"`
	Name            string    `json:"name"`
	SetName         string    `json:"setName"`
	ProductType     string    `json:"productType"`
	MTGStocksID     *int      `json:"mtgstocksId"`
	MarketPrice     *float64  `json:"marketPrice"`
	Quantity        int       `json:"quantity"`
	Weight          float64   `json:"weight"`
	Notes           string    `json:"notes"`
	AddedAt         time.Time `json:"addedAt"`
}

const packColumns = `id, scryfall_set_code, name, set_name, product_type, mtgstocks_id,
	market_price, quantity, weight, COALESCE(notes, ''), added_at`

func scanPack(row interface{ Scan(...any) error }) (*CollectionPack, error) {
	p := &CollectionPack{}
	err := row.Scan(&p.ID, &p.ScryfallSetCode, &p.Name, &p.SetName, &p.ProductType,
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
		p := &CollectionPack{}
		if err := rows.Scan(&p.ID, &p.ScryfallSetCode, &p.Name, &p.SetName, &p.ProductType,
			&p.MTGStocksID, &p.MarketPrice, &p.Quantity, &p.Weight, &p.Notes, &p.AddedAt); err != nil {
			return nil, err
		}
		packs = append(packs, *p)
	}
	return packs, rows.Err()
}

func AddPack(ctx context.Context, pool *pgxpool.Pool, scryfallSetCode, name, setName, productType string, quantity int, weight float64) (*CollectionPack, error) {
	return scanPack(pool.QueryRow(ctx, `
		INSERT INTO collection_packs (scryfall_set_code, name, set_name, product_type, quantity, weight)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+packColumns,
		scryfallSetCode, name, setName, productType, quantity, weight))
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

func GetPacksByIDs(ctx context.Context, pool *pgxpool.Pool, ids []int) ([]CollectionPack, error) {
	rows, err := pool.Query(ctx, `SELECT `+packColumns+` FROM collection_packs WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []CollectionPack
	for rows.Next() {
		p := &CollectionPack{}
		if err := rows.Scan(&p.ID, &p.ScryfallSetCode, &p.Name, &p.SetName, &p.ProductType,
			&p.MTGStocksID, &p.MarketPrice, &p.Quantity, &p.Weight, &p.Notes, &p.AddedAt); err != nil {
			return nil, err
		}
		packs = append(packs, *p)
	}
	return packs, rows.Err()
}

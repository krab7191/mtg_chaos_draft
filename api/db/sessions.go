package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID        string
	UserID    int
	ExpiresAt time.Time
}

func CreateSession(ctx context.Context, pool *pgxpool.Pool, id string, userID int, expiresAt time.Time) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
	`, id, userID, expiresAt)
	return err
}

func GetSession(ctx context.Context, pool *pgxpool.Pool, id string) (*Session, error) {
	s := &Session{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at FROM sessions WHERE id = $1 AND expires_at > NOW()
	`, id).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func DeleteSession(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

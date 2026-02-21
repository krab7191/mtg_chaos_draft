package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       int     `json:"id"`
	GoogleID string  `json:"googleId"`
	Email    string  `json:"email"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Picture  *string `json:"picture,omitempty"`
}

func GetOrCreateUser(ctx context.Context, pool *pgxpool.Pool, googleID, email, name, picture string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		INSERT INTO users (google_id, email, name, picture)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (google_id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name, picture = EXCLUDED.picture
		RETURNING id, google_id, email, name, role, picture
	`, googleID, email, name, picture).Scan(&u.ID, &u.GoogleID, &u.Email, &u.Name, &u.Role, &u.Picture)
	return u, err
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id int) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT id, google_id, email, name, role, picture FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.GoogleID, &u.Email, &u.Name, &u.Role, &u.Picture)
	return u, err
}

func SetUserRole(ctx context.Context, pool *pgxpool.Pool, id int, role string) error {
	_, err := pool.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, id, role)
	return err
}

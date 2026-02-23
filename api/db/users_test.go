package db_test

import (
	"context"
	"testing"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestGetOrCreateUser_Create(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	user, err := db.GetOrCreateUser(ctx, pool, "gid-1", "alice@example.com", "Alice", "https://pic.example.com/a")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email: want %q, got %q", "alice@example.com", user.Email)
	}
	if user.Role != "user" {
		t.Errorf("default role: want %q, got %q", "user", user.Role)
	}
}

func TestGetOrCreateUser_UpdatesExisting(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	_, _ = db.GetOrCreateUser(ctx, pool, "gid-2", "bob@example.com", "Bob", "")
	updated, err := db.GetOrCreateUser(ctx, pool, "gid-2", "bob2@example.com", "Bobby", "https://new-pic")
	if err != nil {
		t.Fatalf("GetOrCreateUser update: %v", err)
	}
	if updated.Email != "bob2@example.com" {
		t.Errorf("updated email: want %q, got %q", "bob2@example.com", updated.Email)
	}
	if updated.Name != "Bobby" {
		t.Errorf("updated name: want %q, got %q", "Bobby", updated.Name)
	}
}

func TestGetUserByID(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	created, _ := db.GetOrCreateUser(ctx, pool, "gid-3", "carol@example.com", "Carol", "")
	got, err := db.GetUserByID(ctx, pool, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("id: want %d, got %d", created.ID, got.ID)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	pool := testhelper.Pool(t)
	_, err := db.GetUserByID(context.Background(), pool, 999999)
	if err == nil {
		t.Error("want error for missing user, got nil")
	}
}

func TestSetUserRole(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	user, _ := db.GetOrCreateUser(ctx, pool, "gid-4", "dan@example.com", "Dan", "")
	if err := db.SetUserRole(ctx, pool, user.ID, "admin"); err != nil {
		t.Fatalf("SetUserRole: %v", err)
	}
	got, _ := db.GetUserByID(ctx, pool, user.ID)
	if got.Role != "admin" {
		t.Errorf("role: want %q, got %q", "admin", got.Role)
	}
}

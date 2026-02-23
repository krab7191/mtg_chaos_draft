package db_test

import (
	"context"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"
)

func TestSessionLifecycle(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	user, _ := db.GetOrCreateUser(ctx, pool, "sess-gid-1", "sess@example.com", "Sess", "")
	expires := time.Now().Add(24 * time.Hour)

	if err := db.CreateSession(ctx, pool, "tok-abc", user.ID, expires); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	session, err := db.GetSession(ctx, pool, "tok-abc")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session.UserID != user.ID {
		t.Errorf("user id: want %d, got %d", user.ID, session.UserID)
	}
	if err := db.DeleteSession(ctx, pool, "tok-abc"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := db.GetSession(ctx, pool, "tok-abc"); err == nil {
		t.Error("want error for deleted session, got nil")
	}
}

func TestGetSession_Expired(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	user, _ := db.GetOrCreateUser(ctx, pool, "sess-gid-2", "expired@example.com", "Ex", "")
	_ = db.CreateSession(ctx, pool, "tok-expired", user.ID, time.Now().Add(-time.Hour))

	if _, err := db.GetSession(ctx, pool, "tok-expired"); err == nil {
		t.Error("want error for expired session, got nil")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "sessions", "users")
	ctx := context.Background()

	user, _ := db.GetOrCreateUser(ctx, pool, "sess-gid-3", "cleanup@example.com", "Clean", "")
	_ = db.CreateSession(ctx, pool, "tok-old", user.ID, time.Now().Add(-time.Hour))
	_ = db.CreateSession(ctx, pool, "tok-new", user.ID, time.Now().Add(time.Hour))

	if err := db.DeleteExpiredSessions(ctx, pool); err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if _, err := db.GetSession(ctx, pool, "tok-old"); err == nil {
		t.Error("expired session should be deleted")
	}
	if _, err := db.GetSession(ctx, pool, "tok-new"); err != nil {
		t.Errorf("valid session should still exist: %v", err)
	}
}

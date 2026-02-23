package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtg-chaos-draft/db"
	mw "mtg-chaos-draft/middleware"
)

// ── RequireAdmin ──────────────────────────────────────────────────────────────

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, &db.User{Role: "admin"}))

	mw.RequireAdmin(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("admin user: want %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRequireAdmin_BlocksViewer(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, &db.User{Role: "viewer"}))

	mw.RequireAdmin(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("viewer user: want %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequireAdmin_BlocksUnknownRole(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, &db.User{Role: "guest"}))

	mw.RequireAdmin(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("unknown role: want %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequireAdmin_BlocksNoUserInContext(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.RequireAdmin(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("no user in context: want %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequireAdmin_NextNotCalledOnForbidden(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, &db.User{Role: "viewer"}))

	mw.RequireAdmin(next).ServeHTTP(rr, req)

	if called {
		t.Error("next handler should not be called when user is not admin")
	}
}

// ── UserFromContext ────────────────────────────────────────────────────────────

func TestUserFromContext_ReturnsUser(t *testing.T) {
	want := &db.User{ID: 7, Email: "admin@example.com", Role: "admin"}
	ctx := context.WithValue(context.Background(), mw.UserContextKey, want)

	got := mw.UserFromContext(ctx)
	if got != want {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestUserFromContext_ReturnsNilWhenMissing(t *testing.T) {
	got := mw.UserFromContext(context.Background())
	if got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestUserFromContext_ReturnsNilForWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), mw.UserContextKey, "not-a-user")
	got := mw.UserFromContext(ctx)
	if got != nil {
		t.Errorf("wrong type in context: want nil, got %v", got)
	}
}

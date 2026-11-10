package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/testhelper"

	"golang.org/x/oauth2"
)

func TestMain(m *testing.M) {
	testhelper.Setup()
	os.Exit(m.Run())
}

// ── loadViewerEmails ──────────────────────────────────────────────────────────

func TestLoadViewerEmails_FromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "viewers")
	if err := os.WriteFile(path, []byte("a@example.com\n  b@example.com \n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadViewerEmails(path, "ignored@example.com")
	want := []string{"a@example.com", "b@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestLoadViewerEmails_FromEnvWhenFileMissing(t *testing.T) {
	got := loadViewerEmails(filepath.Join(t.TempDir(), "missing"), "a@example.com, ,b@example.com")
	want := []string{"a@example.com", "b@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestLoadViewerEmails_None(t *testing.T) {
	if got := loadViewerEmails(filepath.Join(t.TempDir(), "missing"), ""); len(got) != 0 {
		t.Errorf("want none, got %v", got)
	}
}

// ── newRouter ─────────────────────────────────────────────────────────────────

// sessionFor creates a user with the given role and returns a session cookie for it.
func sessionFor(t *testing.T, role string) *http.Cookie {
	t.Helper()
	pool := testhelper.Pool(t)
	ctx := context.Background()
	u, err := db.GetOrCreateUser(ctx, pool, "router-"+role, role+"@example.com", role, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.SetUserRole(ctx, pool, u.ID, role); err != nil {
		t.Fatalf("set role: %v", err)
	}
	sid := "router-session-" + role
	if err := db.CreateSession(ctx, pool, sid, u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "session", Value: sid}
}

func TestRouter_Permissions(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "users")
	oauthCfg := &oauth2.Config{ClientID: "id", Endpoint: oauth2.Endpoint{AuthURL: "https://auth.example.com/auth"}}
	router := newRouter(pool, oauthCfg, "admin@example.com", nil, false)

	admin := sessionFor(t, "admin")
	viewer := sessionFor(t, "viewer")
	user := sessionFor(t, "user")
	stale := &http.Cookie{Name: "session", Value: "does-not-exist"}

	cases := []struct {
		name   string
		method string
		path   string
		cookie *http.Cookie
		want   int
	}{
		{"health is public", http.MethodGet, "/api/health", nil, http.StatusOK},
		{"login redirects to provider", http.MethodGet, "/api/auth/login", nil, http.StatusFound},
		{"me requires a session", http.MethodGet, "/api/auth/me", nil, http.StatusUnauthorized},
		{"unknown session rejected", http.MethodGet, "/api/auth/me", stale, http.StatusUnauthorized},
		{"me for user", http.MethodGet, "/api/auth/me", user, http.StatusOK},
		{"collection for user", http.MethodGet, "/api/collection", user, http.StatusOK},
		{"settings for user", http.MethodGet, "/api/settings", user, http.StatusOK},
		{"drafts forbidden for user", http.MethodGet, "/api/drafts", user, http.StatusForbidden},
		{"drafts for viewer", http.MethodGet, "/api/drafts", viewer, http.StatusOK},
		{"create draft forbidden for viewer", http.MethodPost, "/api/drafts", viewer, http.StatusForbidden},
		{"create draft validates for admin", http.MethodPost, "/api/drafts", admin, http.StatusBadRequest},
		{"delete pack forbidden for user", http.MethodDelete, "/api/collection/1", user, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("not json"))
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Errorf("%s %s: want %d, got %d — %s", tc.method, tc.path, tc.want, rr.Code, rr.Body)
			}
		})
	}
}

// ── Client IP / rate limiting ─────────────────────────────────────────────────

func TestRouter_RateLimitUsesVerifiedClientIP(t *testing.T) {
	router := newRouter(testhelper.Pool(t), &oauth2.Config{}, "", nil, false)
	hit := func(remote, realIP string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.RemoteAddr = remote
		if realIP != "" {
			req.Header.Set("X-Real-IP", realIP)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr.Code
	}

	// Visitor A (forwarded by Caddy on the Docker network) uses up their limit.
	for i := 0; i < 100; i++ {
		if code := hit("172.18.0.5:1234", "203.0.113.10"); code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, code)
		}
	}
	if code := hit("172.18.0.5:1234", "203.0.113.10"); code != http.StatusTooManyRequests {
		t.Fatalf("visitor A over limit: want 429, got %d", code)
	}

	// Visitor B behind the same proxy has their own limit.
	if code := hit("172.18.0.5:1234", "203.0.113.20"); code != http.StatusOK {
		t.Errorf("visitor B: want 200, got %d", code)
	}

	// A direct (non-proxy) client can't dodge the limit by faking X-Real-IP:
	// it is keyed by its own address.
	for i := 0; i < 100; i++ {
		hit("198.51.100.7:999", fmt.Sprintf("10.0.0.%d", i))
	}
	if code := hit("198.51.100.7:999", "10.9.9.9"); code != http.StatusTooManyRequests {
		t.Errorf("spoofing client: want 429, got %d", code)
	}
}

func TestClientIPKey_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:5555"
	if key, err := clientIPKey(req); err != nil || key != "192.0.2.1" {
		t.Errorf("want 192.0.2.1, got %q (%v)", key, err)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mtg-chaos-draft/db"
	mw "mtg-chaos-draft/middleware"
	"mtg-chaos-draft/testhelper"

	"golang.org/x/oauth2"
)

// fakeGoogle serves an OAuth token endpoint and a userinfo endpoint.
type fakeGoogle struct {
	srv          *httptest.Server
	tokenStatus  int
	userInfoBody string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{tokenStatus: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if f.tokenStatus != http.StatusOK {
			http.Error(w, "nope", f.tokenStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, f.userInfoBody)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	orig := googleUserInfoURL
	googleUserInfoURL = f.srv.URL + "/userinfo"
	t.Cleanup(func() { googleUserInfoURL = orig })
	return f
}

func (f *fakeGoogle) config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost:4321/api/auth/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:   f.srv.URL + "/auth",
			TokenURL:  f.srv.URL + "/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

func newAuthTestHandler(t *testing.T, f *fakeGoogle) *AuthHandler {
	t.Helper()
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "users")
	return NewAuthHandler(pool, f.config(), "admin@example.com", []string{"viewer@example.com"}, false)
}

func callbackRequest(state, cookieState string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=abc&state="+url.QueryEscape(state), nil)
	if cookieState != "" {
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: cookieState})
	}
	return req
}

func findCookie(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestAuthLogin_RedirectsWithState(t *testing.T) {
	f := newFakeGoogle(t)
	h := NewAuthHandler(nil, f.config(), "", nil, true)
	rr := httptest.NewRecorder()
	h.Login(rr, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))

	if rr.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rr.Code)
	}
	state := findCookie(rr, "oauth_state")
	if state == nil || state.Value == "" || !state.Secure || !state.HttpOnly {
		t.Fatalf("want secure httponly oauth_state cookie, got %+v", state)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, f.srv.URL+"/auth") || !strings.Contains(loc, "state="+state.Value) {
		t.Errorf("unexpected redirect %q", loc)
	}
}

// ── Callback ──────────────────────────────────────────────────────────────────

func TestAuthCallback_RejectsMissingStateCookie(t *testing.T) {
	f := newFakeGoogle(t)
	h := NewAuthHandler(nil, f.config(), "", nil, false)
	rr := httptest.NewRecorder()
	h.Callback(rr, callbackRequest("s1", ""))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestAuthCallback_RejectsMismatchedState(t *testing.T) {
	f := newFakeGoogle(t)
	h := NewAuthHandler(nil, f.config(), "", nil, false)
	rr := httptest.NewRecorder()
	h.Callback(rr, callbackRequest("s1", "s2"))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestAuthCallback_TokenExchangeFails(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus = http.StatusUnauthorized
	h := NewAuthHandler(nil, f.config(), "", nil, false)
	rr := httptest.NewRecorder()
	h.Callback(rr, callbackRequest("s1", "s1"))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "token exchange") {
		t.Errorf("want 500 token exchange failed, got %d %q", rr.Code, rr.Body)
	}
}

func TestAuthCallback_BadUserInfo(t *testing.T) {
	f := newFakeGoogle(t)
	f.userInfoBody = "not json"
	h := NewAuthHandler(nil, f.config(), "", nil, false)
	rr := httptest.NewRecorder()
	h.Callback(rr, callbackRequest("s1", "s1"))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "decode") {
		t.Errorf("want 500 decode error, got %d %q", rr.Code, rr.Body)
	}
}

func TestAuthCallback_UserInfoUnreachable(t *testing.T) {
	f := newFakeGoogle(t)
	googleUserInfoURL = "http://127.0.0.1:1/userinfo" // nothing listens on port 1
	h := NewAuthHandler(nil, f.config(), "", nil, false)
	rr := httptest.NewRecorder()
	h.Callback(rr, callbackRequest("s1", "s1"))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "userinfo") {
		t.Errorf("want 500 userinfo failed, got %d %q", rr.Code, rr.Body)
	}
}

func TestAuthCallback_SignsInByRole(t *testing.T) {
	cases := []struct {
		email    string
		wantRole string
		wantLoc  string
	}{
		{"admin@example.com", "admin", "/admin/collection"},
		{"viewer@example.com", "viewer", "/admin/collection"},
		{"player@example.com", "user", "/select"},
	}
	for _, tc := range cases {
		t.Run(tc.wantRole, func(t *testing.T) {
			f := newFakeGoogle(t)
			f.userInfoBody = fmt.Sprintf(`{"id":"g-%s","email":%q,"name":"N","picture":""}`, tc.wantRole, tc.email)
			h := newAuthTestHandler(t, f)

			rr := httptest.NewRecorder()
			h.Callback(rr, callbackRequest("s1", "s1"))

			if rr.Code != http.StatusFound || rr.Header().Get("Location") != tc.wantLoc {
				t.Fatalf("want 302 to %s, got %d to %q — %s", tc.wantLoc, rr.Code, rr.Header().Get("Location"), rr.Body)
			}
			c := findCookie(rr, "session")
			if c == nil || c.Value == "" {
				t.Fatal("want session cookie")
			}
			ctx := context.Background()
			sess, err := db.GetSession(ctx, h.pool, c.Value)
			if err != nil {
				t.Fatalf("session not stored: %v", err)
			}
			u, err := db.GetUserByID(ctx, h.pool, sess.UserID)
			if err != nil {
				t.Fatalf("user: %v", err)
			}
			if u.Role != tc.wantRole {
				t.Errorf("role: want %s, got %s", tc.wantRole, u.Role)
			}
		})
	}
}

// ── Logout / Me ───────────────────────────────────────────────────────────────

func TestAuthLogout_DeletesSession(t *testing.T) {
	pool := testhelper.Pool(t)
	testhelper.Truncate(t, pool, "users")
	ctx := context.Background()
	u, err := db.GetOrCreateUser(ctx, pool, "g-logout", "out@example.com", "Out", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(ctx, pool, "logout-sid", u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	h := NewAuthHandler(pool, &oauth2.Config{}, "", nil, false)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "logout-sid"})
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/login" {
		t.Fatalf("want 302 to /login, got %d %q", rr.Code, rr.Header().Get("Location"))
	}
	if _, err := db.GetSession(ctx, pool, "logout-sid"); err == nil {
		t.Error("session should be deleted")
	}
}

func TestAuthLogout_WithoutSession(t *testing.T) {
	h := NewAuthHandler(nil, &oauth2.Config{}, "", nil, false)
	rr := httptest.NewRecorder()
	h.Logout(rr, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if rr.Code != http.StatusFound {
		t.Errorf("want 302, got %d", rr.Code)
	}
}

func TestAuthMe_ReturnsContextUser(t *testing.T) {
	h := NewAuthHandler(nil, &oauth2.Config{}, "", nil, false)
	user := &db.User{ID: 7, Email: "me@example.com", Role: "viewer"}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.UserContextKey, user))
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	var got db.User
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 7 || got.Email != "me@example.com" {
		t.Errorf("unexpected user %+v", got)
	}
}

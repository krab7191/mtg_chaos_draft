package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"mtg-chaos-draft/db"
	mw "mtg-chaos-draft/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	pool         *pgxpool.Pool
	oauthConfig  *oauth2.Config
	adminEmail   string
	viewerEmails []string
}

func NewAuthHandler(pool *pgxpool.Pool, oauthConfig *oauth2.Config, adminEmail string, viewerEmails []string) *AuthHandler {
	return &AuthHandler{pool: pool, oauthConfig: oauthConfig, adminEmail: adminEmail, viewerEmails: viewerEmails}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

	token, err := h.oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	client := h.oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "userinfo failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var info struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		http.Error(w, "decode error", http.StatusInternalServerError)
		return
	}

	user, err := db.GetOrCreateUser(r.Context(), h.pool, info.ID, info.Email, info.Name, info.Picture)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// Promote to admin if email matches ADMIN_EMAIL
	if user.Email == h.adminEmail && user.Role != "admin" {
		if err := db.SetUserRole(r.Context(), h.pool, user.ID, "admin"); err == nil {
			user.Role = "admin"
		}
	}
	// Promote to viewer if email is in VIEWER_EMAILS (and not already admin)
	if user.Role != "admin" {
		for _, email := range h.viewerEmails {
			if user.Email == email && user.Role != "viewer" {
				if err := db.SetUserRole(r.Context(), h.pool, user.ID, "viewer"); err == nil {
					user.Role = "viewer"
				}
				break
			}
		}
	}

	sessionID, err := randomHex(32)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := db.CreateSession(r.Context(), h.pool, sessionID, user.ID, expiresAt); err != nil {
		http.Error(w, "session store error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	adminRoles := []string{"admin", "viewer"}
	for _, role := range adminRoles {
		if user.Role == role {
			http.Redirect(w, r, "/admin/collection", http.StatusFound)
			return
		}
	}
	http.Redirect(w, r, "/select", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session")
	if err == nil {
		if err := db.DeleteSession(r.Context(), h.pool, c.Value); err != nil {
			log.Printf("logout: delete session: %v", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := mw.UserFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

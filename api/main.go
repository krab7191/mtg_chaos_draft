package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	mw "mtg-chaos-draft/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var Version string

func main() {
	log.Printf("starting mtg-chaos-draft %s", Version)

	bgCtx, bgCancel := context.WithCancel(context.Background())

	pool, err := db.New(bgCtx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer pool.Close()

	// Daily price refresh + session cleanup goroutine
	go func() {
		handlers.RefreshPrices(bgCtx, pool)
		if err := db.DeleteExpiredSessions(bgCtx, pool); err != nil {
			log.Printf("session cleanup: %v", err)
		}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handlers.RefreshPrices(bgCtx, pool)
				if err := db.DeleteExpiredSessions(bgCtx, pool); err != nil {
					log.Printf("session cleanup: %v", err)
				}
			case <-bgCtx.Done():
				return
			}
		}
	}()

	redirectURL := mustEnv("GOOGLE_REDIRECT_URL")
	oauthConfig := &oauth2.Config{
		ClientID:     mustEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: mustEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	viewerEmails := loadViewerEmails("/etc/mtg-chaos-draft/viewer_emails", os.Getenv("VIEWER_EMAILS"))
	secureCookies := strings.HasPrefix(redirectURL, "https://")
	r := newRouter(pool, oauthConfig, mustEnv("ADMIN_EMAIL"), viewerEmails, secureCookies)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: r}

	go func() {
		log.Printf("API listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	bgCancel()
	log.Printf("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Printf("shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

// loadViewerEmails reads viewer emails one per line from path, falling back to
// the comma-separated envValue when the file can't be read.
func loadViewerEmails(path, envValue string) []string {
	var raw []string
	if data, err := os.ReadFile(path); err == nil {
		raw = strings.Split(string(data), "\n")
	} else if envValue != "" {
		raw = strings.Split(envValue, ",")
	}
	var emails []string
	for _, e := range raw {
		if trimmed := strings.TrimSpace(e); trimmed != "" {
			emails = append(emails, trimmed)
		}
	}
	return emails
}

// trustedProxyPrefixes are the networks our reverse proxy (Caddy) connects
// from: private Docker networks in production, loopback in local dev.
var trustedProxyPrefixes = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8",
	"::1/128", "fc00::/7",
}

// clientIPKey keys the rate limiter by the client IP resolved by the
// ClientIPFrom* middleware, falling back to the TCP peer address.
func clientIPKey(r *http.Request) (string, error) {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip, nil
	}
	return httprate.KeyByIP(r)
}

// newRouter wires up all HTTP routes and their auth requirements.
func newRouter(pool *pgxpool.Pool, oauthConfig *oauth2.Config, adminEmail string, viewerEmails []string, secureCookies bool) http.Handler {
	authHandler := handlers.NewAuthHandler(pool, oauthConfig, adminEmail, viewerEmails, secureCookies)
	collectionHandler := handlers.NewCollectionHandler(pool)
	settingsHandler := handlers.NewSettingsHandler(pool)
	draftHandler := handlers.NewDraftHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Caddy forwards the verified visitor IP in X-Real-IP. Only trust that
	// header when the request comes from Caddy on the private Docker network;
	// anyone else is identified by their connection address.
	r.Use(middleware.ClientIPFromHeader("X-Real-IP", trustedProxyPrefixes...))
	r.Use(httprate.Limit(100, time.Minute, httprate.WithKeyFuncs(clientIPKey)))

	// Health check
	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Public auth routes
	r.Get("/api/auth/login", authHandler.Login)
	r.Get("/api/auth/callback", authHandler.Callback)
	r.Post("/api/auth/logout", authHandler.Logout)

	// All authenticated users
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth(pool))

		r.Get("/api/auth/me", authHandler.Me)
		r.Get("/api/collection", collectionHandler.List)
		r.Post("/api/collection/{id}/refresh-price", collectionHandler.RefreshPrice)
		r.Get("/api/settings", settingsHandler.Get)

		// Admin/viewer routes (nested)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAdminOrViewer)

			r.Get("/api/search", handlers.Search)
			r.Get("/api/cheapest", handlers.CheapestPacks)
			r.Get("/api/price/{mtgstocksId}", handlers.Price)
			r.Get("/api/drafts", draftHandler.List)
		})

		// Admin-only mutation routes (nested)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAdmin)

			r.Post("/api/collection", collectionHandler.Add)
			r.Put("/api/collection/{id}", collectionHandler.Update)
			r.Post("/api/collection/{id}/link-price", collectionHandler.LinkPrice)
			r.Delete("/api/collection/{id}", collectionHandler.Delete)
			r.Put("/api/settings", settingsHandler.Update)
			r.Post("/api/drafts", draftHandler.Create)
			r.Delete("/api/drafts/{id}", draftHandler.Delete)
			r.Post("/api/drafts/{id}/approve", draftHandler.Approve)
		})
	})
	return r
}

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

	// Daily price refresh goroutine
	go func() {
		handlers.RefreshPrices(bgCtx, pool)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handlers.RefreshPrices(bgCtx, pool)
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

	var viewerEmails []string
	if v := os.Getenv("VIEWER_EMAILS"); v != "" {
		for _, e := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(e); trimmed != "" {
				viewerEmails = append(viewerEmails, trimmed)
			}
		}
	}
	secureCookies := strings.HasPrefix(redirectURL, "https://")
	authHandler := handlers.NewAuthHandler(pool, oauthConfig, mustEnv("ADMIN_EMAIL"), viewerEmails, secureCookies)
	collectionHandler := handlers.NewCollectionHandler(pool)
	selectHandler := handlers.NewSelectHandler(pool)
	settingsHandler := handlers.NewSettingsHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(httprate.LimitByIP(100, time.Minute))

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
		r.Get("/api/search", handlers.Search)
		r.Get("/api/price/{mtgstocksId}", handlers.Price)
		r.Get("/api/collection", collectionHandler.List)
		r.Get("/api/settings", settingsHandler.Get)
		r.Post("/api/select", selectHandler.Select)
	})

	// Admin-only mutation routes
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth(pool))
		r.Use(mw.RequireAdmin)

		r.Post("/api/collection", collectionHandler.Add)
		r.Put("/api/collection/{id}", collectionHandler.Update)
		r.Post("/api/collection/{id}/link-price", collectionHandler.LinkPrice)
		r.Delete("/api/collection/{id}", collectionHandler.Delete)
		r.Put("/api/settings", settingsHandler.Update)
	})

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

package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"mtg-chaos-draft/db"
	"mtg-chaos-draft/handlers"
	mw "mtg-chaos-draft/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	ctx := context.Background()

	pool, err := db.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer pool.Close()

	oauthConfig := &oauth2.Config{
		ClientID:     mustEnv("GOOGLE_CLIENT_ID"),
		ClientSecret: mustEnv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  mustEnv("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	authHandler := handlers.NewAuthHandler(pool, oauthConfig, mustEnv("ADMIN_EMAIL"))
	collectionHandler := handlers.NewCollectionHandler(pool)
	selectHandler := handlers.NewSelectHandler(pool)
	settingsHandler := handlers.NewSettingsHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

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
	log.Printf("API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

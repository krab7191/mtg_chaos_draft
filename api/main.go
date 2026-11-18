package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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

// Overridable in tests: the background jobs and how often they run.
var (
	refreshPrices       = handlers.RefreshPrices
	maintenanceInterval = 24 * time.Hour
	backfillPrices      = handlers.BackfillPriceHistory
	backfillInterval    = time.Hour
)

const viewerEmailsPath = "/etc/mtg-chaos-draft/viewer_emails"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := run(ctx, os.Getenv, viewerEmailsPath); err != nil {
		log.Fatal(err)
	}
	log.Printf("shutdown complete")
}

// run starts the API and blocks until ctx is cancelled (then shuts down
// gracefully) or the server fails.
func run(ctx context.Context, getenv func(string) string, viewerPath string) error {
	log.Printf("starting mtg-chaos-draft %s", Version)

	env, err := requireEnv(getenv, "DATABASE_URL", "GOOGLE_REDIRECT_URL", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "ADMIN_EMAIL")
	if err != nil {
		return err
	}

	pool, err := db.New(ctx, env["DATABASE_URL"])
	if err != nil {
		return fmt.Errorf("db init: %w", err)
	}
	defer pool.Close()

	// Must precede the first price refresh so it can fall back to the saved catalog.
	handlers.UseCatalogStore(pool)

	bgCtx, bgCancel := context.WithCancel(ctx)
	defer bgCancel()
	go runMaintenance(bgCtx, pool, maintenanceInterval)
	go runBackfill(bgCtx, pool, backfillInterval)

	redirectURL := env["GOOGLE_REDIRECT_URL"]
	oauthConfig := &oauth2.Config{
		ClientID:     env["GOOGLE_CLIENT_ID"],
		ClientSecret: env["GOOGLE_CLIENT_SECRET"],
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	handlers.ConfigureNotifications(getenv("NTFY_URL"), siteOrigin(redirectURL))

	viewerEmails := loadViewerEmails(viewerPath, getenv("VIEWER_EMAILS"))
	secureCookies := strings.HasPrefix(redirectURL, "https://")
	r := newRouter(pool, oauthConfig, env["ADMIN_EMAIL"], viewerEmails, secureCookies)

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 10 * time.Second}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("API listening on :%s", port)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	bgCancel()
	log.Printf("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// runMaintenance refreshes prices and clears expired sessions now and then
// every interval, until ctx is cancelled.
func runMaintenance(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		refreshPrices(ctx, pool)
		if err := db.DeleteExpiredSessions(ctx, pool); err != nil && ctx.Err() == nil {
			log.Printf("session cleanup: %v", err)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// runBackfill fetches price history for products that don't have it yet,
// now and then every interval: after a restart, a tunnel outage, or when a
// new set appears. Once everything is backfilled each run is a no-op.
func runBackfill(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := backfillPrices(ctx, pool); err != nil && ctx.Err() == nil {
			log.Printf("price history backfill paused: %v", err)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// siteOrigin is the scheme and host of a URL, e.g. the public site from the
// OAuth redirect URL; empty if it can't be parsed.
func siteOrigin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// requireEnv reads the named variables, failing if any is empty.
func requireEnv(getenv func(string) string, keys ...string) (map[string]string, error) {
	env := make(map[string]string, len(keys))
	var missing []string
	for _, k := range keys {
		if env[k] = getenv(k); env[k] == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return env, nil
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
// ClientIPFrom* middleware (which only trusts X-Real-IP from Caddy), falling
// back to the TCP peer address. IPv6 clients are bucketed per /64.
func clientIPKey(r *http.Request) (string, error) {
	ip := middleware.GetClientIP(r.Context())
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip = host
	}
	return httprate.CanonicalizeIP(ip), nil
}

// newRouter wires up all HTTP routes and their auth requirements.
func newRouter(pool *pgxpool.Pool, oauthConfig *oauth2.Config, adminEmail string, viewerEmails []string, secureCookies bool) http.Handler {
	authHandler := handlers.NewAuthHandler(pool, oauthConfig, adminEmail, viewerEmails, secureCookies)
	collectionHandler := handlers.NewCollectionHandler(pool)
	settingsHandler := handlers.NewSettingsHandler(pool)
	draftHandler := handlers.NewDraftHandler(pool)
	purchaseHandler := handlers.NewPurchaseHandler(pool)
	saleHandler := handlers.NewSaleHandler(pool)
	catalogHandler := handlers.NewCatalogHandler(pool)
	trendsHandler := handlers.NewTrendsHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Caddy forwards the verified visitor IP in X-Real-IP. Only trust that
	// header when the request comes from Caddy on the private Docker network;
	// anyone else is identified by their connection address.
	r.Use(middleware.ClientIPFromHeader("X-Real-IP", trustedProxyPrefixes...))
	r.Use(httprate.LimitBy(100, time.Minute, clientIPKey))

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
			r.Get("/api/catalog", catalogHandler.Status)
			r.Get("/api/trends", trendsHandler.List)
			r.Get("/api/trends/{mtgstocksId}/history", trendsHandler.History)
			r.Get("/api/cheapest", handlers.CheapestPacks)
			r.Get("/api/price/{mtgstocksId}", handlers.Price)
			r.Get("/api/drafts", draftHandler.List)
			r.Get("/api/purchases", purchaseHandler.List)
			r.Get("/api/disposals", saleHandler.List)
		})

		// Admin-only mutation routes (nested)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAdmin)

			r.Post("/api/collection", collectionHandler.Add)
			r.Put("/api/collection/{id}", collectionHandler.Update)
			r.Post("/api/collection/{id}/link-price", collectionHandler.LinkPrice)
			r.Delete("/api/collection/{id}", collectionHandler.Delete)
			r.Post("/api/purchases", purchaseHandler.Create)
			r.Put("/api/purchases/{id}", purchaseHandler.Update)
			r.Delete("/api/purchases/{id}", purchaseHandler.Delete)
			r.Post("/api/sales", saleHandler.Create)
			r.Put("/api/sales/{id}", saleHandler.Update)
			r.Delete("/api/sales/{id}", saleHandler.Delete)
			r.Put("/api/drafts/{id}/collected", saleHandler.SetDraftCollected)
			r.Put("/api/settings", settingsHandler.Update)
			r.Put("/api/catalog", catalogHandler.Upload)
			r.Put("/api/trends/watch", trendsHandler.SetWatchMany)
			r.Post("/api/trends/notify-test", trendsHandler.NotifyTest)
			r.Put("/api/trends/{mtgstocksId}/watch", trendsHandler.SetWatch)
			r.Post("/api/drafts", draftHandler.Create)
			r.Delete("/api/drafts/{id}", draftHandler.Delete)
			r.Post("/api/drafts/{id}/approve", draftHandler.Approve)
		})
	})
	return r
}

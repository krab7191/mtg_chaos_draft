package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"mtg-chaos-draft/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Phone alerts through ntfy (https://ntfy.sh): the server POSTs a message to
// a topic URL and the ntfy app on the phone shows it. Free, no account; the
// topic name is the secret, so it should be long and random.

var (
	ntfyURL    string // e.g. https://ntfy.sh/mtg-<random>; empty disables alerts
	siteURL    string // where a tapped notification opens, e.g. https://example.com
	ntfyClient = &http.Client{Timeout: 10 * time.Second}
)

// ConfigureNotifications sets where phone alerts go and which site they link to.
func ConfigureNotifications(topicURL, site string) {
	ntfyURL = strings.TrimSpace(topicURL)
	siteURL = strings.TrimRight(strings.TrimSpace(site), "/")
	if ntfyURL != "" {
		log.Printf("price alerts: phone notifications on")
	}
}

func notificationsOn() bool { return ntfyURL != "" }

// sendNotification posts one message to the ntfy topic.
func sendNotification(ctx context.Context, title, message string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ntfyURL, strings.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Tags", "chart_with_downwards_trend")
	if siteURL != "" {
		req.Header.Set("Click", siteURL+"/admin/trends")
	}
	resp, err := ntfyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned %d", resp.StatusCode)
	}
	return nil
}

// alertMessage words one alert for the phone.
func alertMessage(p SearchResult, t *Trend, w *db.Watch, reason string) (title, body string) {
	name := p.SetName + " " + p.ProductType
	price := fmt.Sprintf("$%.2f", t.Current)
	if p.PacksPerBox > 0 {
		price += fmt.Sprintf(" ($%.2f/pack)", t.Current/float64(p.PacksPerBox))
	}
	since := fmt.Sprintf("%+.0f%% since release", -t.DropFromRelease*100)
	if reason == AlertDrop && w.DropAlert != nil {
		return "Price drop: " + name,
			fmt.Sprintf("%s, %s (your alert: -%.0f%%). Low since release $%.2f.", price, since, *w.DropAlert*100, t.Low)
	}
	return "Possible floor: " + name,
		fmt.Sprintf("%s, %s. Steady near its low ($%.2f) for %d days.", price, since, t.Low, floorWindow)
}

// watchedAlert is a watched product with its current trend and alerts.
type watchedAlert struct {
	product SearchResult
	trend   *Trend
	watch   db.Watch
	alerts  []string
}

// currentAlerts works out today's alerts for watched products (all of them,
// or only those in ids).
func currentAlerts(ctx context.Context, pool *pgxpool.Pool, ids []int) ([]watchedAlert, error) {
	watches, err := db.ListWatches(ctx, pool)
	if err != nil || len(watches) == 0 {
		return nil, err
	}
	only := map[int]bool{}
	for _, id := range ids {
		only[id] = true
	}
	c, err := getCatalog()
	if err != nil {
		return nil, err
	}
	var picked []SearchResult
	var pickedIDs []int
	for _, p := range trackedProducts(c) {
		if _, ok := watches[p.MTGStocksID]; ok && (ids == nil || only[p.MTGStocksID]) {
			picked = append(picked, p)
			pickedIDs = append(pickedIDs, p.MTGStocksID)
		}
	}
	history, err := db.PriceHistory(ctx, pool, pickedIDs, time.Time{})
	if err != nil {
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	out := make([]watchedAlert, 0, len(picked))
	for _, p := range picked {
		var released *time.Time
		if rel, err := time.Parse("2006-01-02", p.ReleasedAt); err == nil {
			released = &rel
		}
		w := watches[p.MTGStocksID]
		t := summarize(history[p.MTGStocksID], released, today)
		out = append(out, watchedAlert{product: p, trend: t, watch: w, alerts: alertsFor(t, &w)})
	}
	return out, nil
}

// alertLine is one alert in a summary notification.
func alertLine(a watchedAlert, reason string) string {
	t := a.trend
	since := fmt.Sprintf("%+.0f%% since release", -t.DropFromRelease*100)
	name := a.product.SetName + " " + a.product.ProductType
	if reason == AlertDrop && a.watch.DropAlert != nil {
		return fmt.Sprintf("↓ %s: $%.2f, %s (alert -%.0f%%)", name, t.Current, since, *a.watch.DropAlert*100)
	}
	return fmt.Sprintf("◆ %s: possible floor at $%.2f, %s", name, t.Current, since)
}

// NotifyAlerts sends the phone what's new since the last check: one
// notification for a single new alert, or one summary for several, so a
// busy day doesn't buzz the phone over and over. Alerts that cleared are
// forgotten so they can fire again later. Returns notifications sent.
func NotifyAlerts(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	if !notificationsOn() {
		return 0, nil
	}
	current, err := currentAlerts(ctx, pool, nil)
	if err != nil {
		return 0, err
	}

	type pending struct {
		a      watchedAlert
		reason string
	}
	var news []pending
	for _, a := range current {
		for _, reason := range a.alerts {
			if !slices.Contains(a.watch.Notified, reason) {
				news = append(news, pending{a, reason})
			}
		}
	}

	sent := 0
	delivered := len(news) == 0
	switch {
	case len(news) == 1:
		title, body := alertMessage(news[0].a.product, news[0].a.trend, &news[0].a.watch, news[0].reason)
		delivered = sendNotification(ctx, title, body) == nil
	case len(news) > 1:
		lines := make([]string, len(news))
		for i, n := range news {
			lines[i] = alertLine(n.a, n.reason)
		}
		delivered = sendNotification(ctx, fmt.Sprintf("%d new price alerts", len(news)), strings.Join(lines, "\n")) == nil
	}
	if len(news) > 0 {
		if !delivered {
			log.Printf("price alerts: sending %d new alerts failed; retrying next run", len(news))
		} else {
			sent = 1
			log.Printf("price alerts: sent %d new alerts", len(news))
		}
	}

	// Remember what the phone has been told: everything current once
	// delivered; otherwise only what it already knew (minus cleared alerts).
	for _, a := range current {
		known := a.alerts
		if !delivered {
			known = []string{}
			for _, r := range a.alerts {
				if slices.Contains(a.watch.Notified, r) {
					known = append(known, r)
				}
			}
		}
		if !slices.Equal(known, a.watch.Notified) {
			if err := db.SetNotified(ctx, pool, a.watch.MTGStocksID, known); err != nil {
				return sent, err
			}
		}
	}
	return sent, nil
}

// markAlertsSeen records the current alerts of newly (re)configured watches
// as already delivered: the page shows them right away, so the phone only
// hears about alerts that appear later. Best effort; failures are logged.
func markAlertsSeen(ctx context.Context, pool *pgxpool.Pool, ids []int) {
	current, err := currentAlerts(ctx, pool, ids)
	if err != nil {
		log.Printf("price alerts: marking seen: %v", err)
		return
	}
	for _, a := range current {
		if !slices.Equal(a.alerts, a.watch.Notified) {
			if err := db.SetNotified(ctx, pool, a.watch.MTGStocksID, a.alerts); err != nil {
				log.Printf("price alerts: marking seen: %v", err)
				return
			}
		}
	}
}

// NotifyTest sends a test alert so the phone setup can be checked.
func (h *TrendsHandler) NotifyTest(w http.ResponseWriter, r *http.Request) {
	if !notificationsOn() {
		http.Error(w, "phone alerts aren't set up: add NTFY_URL to the server's .env", http.StatusBadRequest)
		return
	}
	if err := sendNotification(r.Context(), "MTG Chaos Draft: test alert", "Phone alerts work. Watched packs and boxes will notify you here."); err != nil {
		http.Error(w, "couldn't reach ntfy: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

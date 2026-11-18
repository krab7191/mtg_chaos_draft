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

// NotifyAlerts sends a phone alert for each watched product with an alert
// that hasn't been sent yet, and forgets alerts that cleared so they can fire
// again. Returns how many notifications were sent.
func NotifyAlerts(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	if !notificationsOn() {
		return 0, nil
	}
	watches, err := db.ListWatches(ctx, pool)
	if err != nil || len(watches) == 0 {
		return 0, err
	}
	c, err := getCatalog()
	if err != nil {
		return 0, err
	}
	products := map[int]SearchResult{}
	var ids []int
	for _, p := range trackedProducts(c) {
		if _, ok := watches[p.MTGStocksID]; ok {
			products[p.MTGStocksID] = p
			ids = append(ids, p.MTGStocksID)
		}
	}
	history, err := db.PriceHistory(ctx, pool, ids, time.Time{})
	if err != nil {
		return 0, err
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	sent := 0
	for _, id := range ids {
		p, w := products[id], watches[id]
		var released *time.Time
		if rel, err := time.Parse("2006-01-02", p.ReleasedAt); err == nil {
			released = &rel
		}
		t := summarize(history[id], released, today)
		alerts := alertsFor(t, &w)

		delivered := []string{}
		for _, reason := range alerts {
			if slices.Contains(w.Notified, reason) {
				delivered = append(delivered, reason)
				continue
			}
			title, body := alertMessage(p, t, &w, reason)
			if err := sendNotification(ctx, title, body); err != nil {
				log.Printf("price alerts: sending %q: %v", title, err)
				continue // retried next run
			}
			delivered = append(delivered, reason)
			sent++
		}
		if !slices.Equal(delivered, w.Notified) {
			if err := db.SetNotified(ctx, pool, id, delivered); err != nil {
				return sent, err
			}
		}
	}
	if sent > 0 {
		log.Printf("price alerts: sent %d", sent)
	}
	return sent, nil
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

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var mtgstocksClient = newMTGStocksClient(os.Getenv("MTGSTOCKS_PROXY"))

// newMTGStocksClient returns the client for MTGStocks requests. When proxyURL
// is set (e.g. an HTTP proxy reached over WireGuard), only these requests go
// through it; everything else the server does is unaffected.
func newMTGStocksClient(proxyURL string) *http.Client {
	c := &http.Client{Timeout: 10 * time.Second}
	if proxyURL == "" {
		return c
	}
	u, err := url.Parse(proxyURL)
	if err != nil || u.Host == "" {
		log.Printf("mtgstocks: ignoring invalid MTGSTOCKS_PROXY %q", proxyURL)
		return c
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyURL(u)
	c.Transport = t
	log.Printf("mtgstocks: requests go through proxy %s", u.Host)
	return c
}

func proxyMTGStocks(url string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 500, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.mtgstocks.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.mtgstocks.com")

	resp, err := mtgstocksClient.Do(req)
	if err != nil {
		return nil, 502, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// fetchSealedPrice returns the market price for an MTGStocks sealed product:
// live when MTGStocks answers, otherwise from the cached catalog. Returns
// 0 and false if neither has a price.
func fetchSealedPrice(mtgstocksID int) (float64, bool) {
	if price, ok := fetchLiveSealedPrice(mtgstocksID); ok {
		return price, true
	}
	return cachedSealedPrice(mtgstocksID)
}

func fetchLiveSealedPrice(mtgstocksID int) (float64, bool) {
	body, status, err := proxyMTGStocks(fmt.Sprintf("https://api.mtgstocks.com/sealed/%d", mtgstocksID))
	if err != nil || status != 200 {
		return 0, false
	}

	var data struct {
		LatestPrice *struct {
			Market  *float64 `json:"market"`
			Average *float64 `json:"average"`
			Low     *float64 `json:"low"`
		} `json:"latestPrice"`
	}
	if err := json.Unmarshal(body, &data); err != nil || data.LatestPrice == nil {
		return 0, false
	}

	lp := data.LatestPrice
	return pickPrice(lp.Low, lp.Market, lp.Average)
}

// pickPrice returns the first positive price in order of preference. We use
// TCGplayer's MARKET price (recent completed sales) — it matches what
// TCGplayer shows and what it costs to restock. Average listing price, then
// the single lowest listing, are fallbacks only.
func pickPrice(low, market, average *float64) (float64, bool) {
	for _, v := range []*float64{market, average, low} {
		if v != nil && *v > 0 {
			return *v, true
		}
	}
	return 0, false
}

func Price(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("mtgstocksId")
	body, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/" + id)
	if err != nil {
		http.Error(w, "couldn't reach MTGStocks — try again shortly", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

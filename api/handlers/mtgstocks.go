package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var mtgstocksClient = &http.Client{}

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

// fetchSealedPrice fetches the market price for an MTGStocks sealed product.
// Returns the price and true on success, or 0 and false if unavailable.
func fetchSealedPrice(mtgstocksID int) (float64, bool) {
	body, status, err := proxyMTGStocks(fmt.Sprintf("https://api.mtgstocks.com/sealed/%d", mtgstocksID))
	if err != nil || status != 200 {
		fmt.Printf("fetchSealedPrice %d: status=%d err=%v\n", mtgstocksID, status, err)
		return 0, false
	}
	fmt.Printf("fetchSealedPrice %d raw: %s\n", mtgstocksID, string(body))

	var data map[string]json.RawMessage
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, false
	}

	// Try every plausible top-level numeric field
	for _, key := range []string{"avg", "market", "low", "high", "price", "market_price", "latest_price"} {
		raw, ok := data[key]
		if !ok {
			continue
		}
		var v float64
		if err := json.Unmarshal(raw, &v); err == nil && v > 0 {
			fmt.Printf("fetchSealedPrice %d: using field %q = %.2f\n", mtgstocksID, key, v)
			return v, true
		}
	}
	fmt.Printf("fetchSealedPrice %d: no usable price field found\n", mtgstocksID)
	return 0, false
}

func Price(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("mtgstocksId")
	body, status, err := proxyMTGStocks("https://api.mtgstocks.com/sealed/" + id)
	if err != nil {
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

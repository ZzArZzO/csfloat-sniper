package prices

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const steamPriceOverviewURL = "https://steamcommunity.com/market/priceoverview/"

type steamClient struct {
	http *http.Client
}

func newSteamClient() *steamClient {
	return &steamClient{http: &http.Client{Timeout: 8 * time.Second}}
}

// fetchVolume returns the 24h sales volume for a CS2 item from Steam Market.
// Returns 0 on any error.
func (s *steamClient) fetchVolume(marketHashName string) int {
	req, err := http.NewRequest(http.MethodGet, steamPriceOverviewURL, nil)
	if err != nil {
		return 0
	}
	q := url.Values{}
	q.Set("appid", "730")
	q.Set("currency", "1")
	q.Set("market_hash_name", marketHashName)
	req.URL.RawQuery = q.Encode()

	resp, err := s.http.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}

	var data struct {
		Success bool   `json:"success"`
		Volume  string `json:"volume"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || !data.Success {
		return 0
	}

	// Steam returns volume as "1,234" — strip commas before parsing.
	clean := strings.ReplaceAll(data.Volume, ",", "")
	v, err := strconv.Atoi(clean)
	if err != nil {
		return 0
	}
	return v
}

package prices

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const cs2shURL = "https://api.cs2.sh/v1/prices/latest"

type cs2shClient struct {
	apiKey     string
	httpClient *http.Client
}

func newCS2ShClient(apiKey string) *cs2shClient {
	return &cs2shClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// fetchAll fetches all item prices in one request and returns a map of
// market_hash_name → PriceData with Buff and Youpin asks converted to cents.
func (c *cs2shClient) fetchAll() (map[string]PriceData, error) {
	req, err := http.NewRequest(http.MethodGet, cs2shURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cs2.sh returned %d", resp.StatusCode)
	}

	var result struct {
		Items map[string]struct {
			Buff   *struct {
				Ask    float64 `json:"ask"`
				Volume int     `json:"volume"`
			} `json:"buff"`
			Youpin *struct {
				Ask    float64 `json:"ask"`
				Volume int     `json:"volume"`
			} `json:"youpin"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out := make(map[string]PriceData, len(result.Items))
	for name, item := range result.Items {
		var pd PriceData
		if item.Buff != nil && item.Buff.Ask > 0 {
			pd.BuffAsk = int(math.Round(item.Buff.Ask * 100))
			if item.Buff.Volume > pd.Volume {
				pd.Volume = item.Buff.Volume
			}
		}
		if item.Youpin != nil && item.Youpin.Ask > 0 {
			pd.YoupinAsk = int(math.Round(item.Youpin.Ask * 100))
			if item.Youpin.Volume > pd.Volume {
				pd.Volume = item.Youpin.Volume
			}
		}
		if pd.BuffAsk > 0 || pd.YoupinAsk > 0 {
			out[name] = pd
		}
	}
	return out, nil
}

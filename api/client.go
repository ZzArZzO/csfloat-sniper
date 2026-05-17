package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	csfloat "github.com/Bios-Marcel/csfloat_go"
)

type Client struct {
	poll         []*csfloat.API
	buy          *csfloat.API
	idx          atomic.Uint64
	loggedLimits atomic.Bool
}

func NewClient(mainKey string, pollKeys []string, proxies []string) *Client {
	c := &Client{
		buy: csfloat.New(mainKey),
	}
	for i, k := range pollKeys {
		var proxy string
		if len(proxies) > 0 {
			proxy = proxies[i%len(proxies)]
		}
		c.poll = append(c.poll, newAPIClient(k, proxy))
	}
	// Fall back to main key for polling when no secondary keys are configured.
	if len(c.poll) == 0 {
		c.poll = append(c.poll, c.buy)
	}
	if len(proxies) > 0 {
		log.Printf("key pool: %d poll key(s), %d proxy/proxies, dedicated buy key", len(c.poll), len(proxies))
	} else {
		log.Printf("key pool: %d poll key(s), no proxies, dedicated buy key", len(c.poll))
	}
	return c
}

// newAPIClient creates a csfloat API client, optionally routing through a proxy.
// proxyURL may be empty (direct connection) or an http/https/socks5 URL.
func newAPIClient(apiKey, proxyURL string) *csfloat.API {
	if proxyURL == "" {
		return csfloat.New(apiKey)
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		log.Printf("proxy: invalid URL %q, using direct connection: %v", proxyURL, err)
		return csfloat.New(apiKey)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(parsed),
		DialContext: (&net.Dialer{
			Timeout: 15 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 3 * time.Second,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	return csfloat.NewWithHTTPClient(apiKey, httpClient)
}

// rateLimitBuffer is how many requests we keep in reserve per key.
// A key is retired once its remaining count drops below this threshold,
// so we never actually hit the hard limit.
const rateLimitBuffer = 5

func (c *Client) FetchNewest() ([]csfloat.ActiveListing, error) {
	n := len(c.poll)
	base := int(c.idx.Add(1)-1) % n

	var soonestReset time.Time
	skipped := 0

	for attempt := 0; attempt < n; attempt++ {
		keyIdx := (base + attempt) % n
		api := c.poll[keyIdx]

		// Pre-check: if we already know this key is at or below the buffer,
		// skip it without making a request. Once its reset time passes, the
		// cached info is stale and we allow it again.
		if rl := api.LastRatelimit(); rl != nil && time.Now().Before(rl.Reset) && rl.Remaining < rateLimitBuffer {
			skipped++
			log.Printf("key %d: %d req remaining (buffer=%d), skipping until %v", keyIdx, rl.Remaining, rateLimitBuffer, rl.Reset.Format("15:04:05"))
			if soonestReset.IsZero() || rl.Reset.Before(soonestReset) {
				soonestReset = rl.Reset
			}
			continue
		}

		resp, err := api.Listings(csfloat.ListingsRequest{
			SortBy: csfloat.Newest,
			Type:   csfloat.BuyNow,
		})
		rl := api.LastRatelimit()

		if err != nil {
			if rl != nil && rl.Remaining < rateLimitBuffer {
				skipped++
				log.Printf("key %d: hit buffer after request (%d remaining), trying next key", keyIdx, rl.Remaining)
				if soonestReset.IsZero() || rl.Reset.Before(soonestReset) {
					soonestReset = rl.Reset
				}
				continue
			}
			return nil, err
		}

		if rl != nil && c.loggedLimits.CompareAndSwap(false, true) {
			log.Printf("rate limit: %d req/window per key, resets at %v", rl.Limit, rl.Reset.Format("15:04:05"))
		}
		if rl != nil && rl.Remaining < rateLimitBuffer {
			log.Printf("key %d: %d/%d remaining — retiring until %v", keyIdx, rl.Remaining, rl.Limit, rl.Reset.Format("15:04:05"))
		}

		return resp.Data, nil
	}

	// All keys are within the safety buffer — sleep until the soonest reset.
	if !soonestReset.IsZero() {
		wait := time.Until(soonestReset) + time.Second
		if wait > 0 {
			log.Printf("all %d poll keys within safety buffer — sleeping %v until reset", skipped, wait.Round(time.Second))
			time.Sleep(wait)
		}
	}
	return nil, fmt.Errorf("all poll keys within safety buffer")
}

func (c *Client) BuyListing(contractID string, price int) error {
	_, err := c.buy.Buy(csfloat.BuyRequestPayload{
		ContractIds: []string{contractID},
		TotalPrice:  uint(price),
	})
	return err
}

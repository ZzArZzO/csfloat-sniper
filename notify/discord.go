package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/user/csfloat-sniper/engine"
)

// Discord sends deal alerts as rich embeds to a Discord webhook.
type Discord struct {
	webhookURL string
	httpClient *http.Client
}

func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify fires a webhook in a background goroutine so the poller is never blocked.
func (d *Discord) Notify(deal engine.Deal) {
	go d.send(deal)
}

func (d *Discord) send(deal engine.Deal) {
	payload, err := json.Marshal(map[string]any{"embeds": []any{buildEmbed(deal)}})
	if err != nil {
		return
	}
	resp, err := d.httpClient.Post(d.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("discord: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("discord: webhook returned %d", resp.StatusCode)
	}
}

// ── Embed construction ────────────────────────────────────────────────────────

type discordEmbed struct {
	Title     string         `json:"title"`
	URL       string         `json:"url"`
	Color     int            `json:"color"`
	Fields    []embedField   `json:"fields"`
	Thumbnail *embedImage    `json:"thumbnail,omitempty"`
	Footer    *embedFooter   `json:"footer,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type embedImage struct {
	URL string `json:"url"`
}

type embedFooter struct {
	Text string `json:"text"`
}

var strategyColor = map[string]int{
	"price":   0x22c55e, // green
	"float":   0x38bdf8, // sky blue
	"pattern": 0xa855f7, // purple
}

func buildEmbed(deal engine.Deal) discordEmbed {
	color := strategyColor[deal.MatchedBy]
	if color == 0 {
		color = 0xffffff
	}

	title := fmt.Sprintf("[%s] %s (%s)", strings.ToUpper(deal.MatchedBy), deal.ItemName, deal.Condition)

	var fields []embedField

	fields = append(fields, embedField{
		Name:   "Price",
		Value:  fmt.Sprintf("$%.2f", float64(deal.Price)/100),
		Inline: true,
	})

	if deal.Discount > 0 {
		fields = append(fields, embedField{
			Name:   "Discount",
			Value:  fmt.Sprintf("%.1f%%", deal.Discount),
			Inline: true,
		})
	}

	if deal.Float > 0 {
		fields = append(fields, embedField{
			Name:   "Float",
			Value:  fmt.Sprintf("%.7f", deal.Float),
			Inline: true,
		})
	}

	if deal.PaintSeed > 0 {
		fields = append(fields, embedField{
			Name:   "Seed",
			Value:  fmt.Sprintf("%d", deal.PaintSeed),
			Inline: true,
		})
	}

	if deal.BuffPrice > 0 {
		fields = append(fields, embedField{
			Name:   "Buff163",
			Value:  fmt.Sprintf("$%.2f", float64(deal.BuffPrice)/100),
			Inline: true,
		})
	}

	if deal.YoupinPrice > 0 {
		fields = append(fields, embedField{
			Name:   "Youpin",
			Value:  fmt.Sprintf("$%.2f", float64(deal.YoupinPrice)/100),
			Inline: true,
		})
	}

	if deal.StickerSP > 0 {
		fields = append(fields, embedField{
			Name:   "Sticker SP%",
			Value:  fmt.Sprintf("%.1f%%", deal.StickerSP),
			Inline: true,
		})
	}

	e := discordEmbed{
		Title:     title,
		URL:       deal.ListingURL,
		Color:     color,
		Fields:    fields,
		Footer:    &embedFooter{Text: "CSFloat Sniper"},
		Timestamp: deal.DetectedAt.UTC().Format(time.RFC3339),
	}

	if deal.IconURL != "" {
		iconURL := deal.IconURL
		if !strings.HasPrefix(iconURL, "http") {
			iconURL = "https://community.cloudflare.steamstatic.com/economy/image/" + iconURL + "/48fx36f"
		}
		e.Thumbnail = &embedImage{URL: iconURL}
	}

	return e
}

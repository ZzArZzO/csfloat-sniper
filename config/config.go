package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// Filters are global pre-conditions applied before any snipe rule is checked.
// All monetary values are in USD. Zero means the filter is disabled.
type Filters struct {
	MinPrice        float64 `yaml:"min_price"`        // skip listings cheaper than this
	MaxPrice        float64 `yaml:"max_price"`        // skip listings more expensive than this
	MinSaving       float64 `yaml:"min_saving"`       // skip deals saving less than this vs ref price
	MinVolume        int     `yaml:"min_volume"`         // skip items with fewer 24h Steam sales
	MinStickerValue  float64 `yaml:"min_sticker_value"`  // ignore stickers worth less than this total ($)
	ExcludeStatTrak  bool    `yaml:"exclude_stattrak"`   // ignore all StatTrak™ items
	ExcludeSouvenir  bool    `yaml:"exclude_souvenir"`   // ignore all Souvenir items
}

type PriceSnipe struct {
	Enabled        bool    `yaml:"enabled"`
	MinDiscountPct float64 `yaml:"min_discount_pct"`
}

type FloatRule struct {
	Item     string  `yaml:"item"`
	MaxFloat float64 `yaml:"max_float"`
}

type FloatSnipe struct {
	Enabled bool        `yaml:"enabled"`
	Rules   []FloatRule `yaml:"rules"`
}

type PatternRule struct {
	Item  string `yaml:"item"`
	Seeds []uint `yaml:"seeds"`
}

type PatternSnipe struct {
	Enabled bool          `yaml:"enabled"`
	Rules   []PatternRule `yaml:"rules"`
}

type Config struct {
	// MainAPIKey is your main CSFloat account key — used exclusively for buying.
	MainAPIKey string `yaml:"main_api_key"`
	// PollAPIKeys are secondary account keys used for polling (round-robin).
	// If empty, MainAPIKey is used for polling too (single-key mode).
	PollAPIKeys  []string     `yaml:"poll_api_keys"`
	PollInterval Duration     `yaml:"poll_interval"`
	PollJitter   Duration     `yaml:"poll_jitter"`
	// Proxies is an optional list of proxy URLs (http/https/socks5) assigned
	// to poll keys round-robin. Leave empty for direct connections.
	Proxies      []string     `yaml:"proxies"`
	ServerPort   int          `yaml:"server_port"`
	Filters      Filters      `yaml:"filters"`
	PriceSnipe   PriceSnipe   `yaml:"price_snipe"`
	FloatSnipe   FloatSnipe   `yaml:"float_snipe"`
	PatternSnipe PatternSnipe `yaml:"pattern_snipe"`
	// CS2CapAPIKey enables Buff163+Youpin price lookup via CS2Cap.
	// Leave empty to fall back to CSFloat's predicted_price.
	CS2CapAPIKey         string   `yaml:"cs2cap_api_key"`
	PriceRefreshInterval Duration `yaml:"price_refresh_interval"`
	// DiscordWebhookURL sends deal alerts to a Discord channel.
	// Leave empty to disable.
	DiscordWebhookURL string `yaml:"discord_webhook_url"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	cfg.PollInterval.Duration = 20 * time.Second
	cfg.PriceRefreshInterval.Duration = 30 * time.Minute
	cfg.ServerPort = 8080
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

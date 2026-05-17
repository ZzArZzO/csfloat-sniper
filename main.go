package main

import (
	"database/sql"
	"log"

	"github.com/user/csfloat-sniper/api"
	"github.com/user/csfloat-sniper/config"
	"github.com/user/csfloat-sniper/engine"
	"github.com/user/csfloat-sniper/notify"
	"github.com/user/csfloat-sniper/prices"
	"github.com/user/csfloat-sniper/server"
	_ "modernc.org/sqlite"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlDB, err := sql.Open("sqlite", "sniper.db")
	if err != nil {
		log.Fatalf("db: open: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec("PRAGMA synchronous=NORMAL")

	var priceDB *prices.DB
	if cfg.CS2CapAPIKey != "" {
		priceDB = prices.NewDB(cfg.CS2CapAPIKey, cfg.PriceRefreshInterval.Duration, sqlDB)
		priceDB.Start()
	} else {
		log.Printf("prices: no cs2cap_api_key set — falling back to CSFloat predicted_price")
	}

	var notifiers []engine.Notifier
	if cfg.DiscordWebhookURL != "" {
		notifiers = append(notifiers, notify.NewDiscord(cfg.DiscordWebhookURL))
		log.Printf("discord: notifications enabled")
	}

	client := api.NewClient(cfg.MainAPIKey, cfg.PollAPIKeys, cfg.Proxies)
	store := engine.NewStore(sqlDB)
	broadcaster := engine.NewBroadcaster()
	filter := engine.NewFilter(cfg, priceDB)
	poller := engine.NewPoller(client, filter, store, broadcaster, cfg, notifiers...)

	go poller.Run()

	log.Printf("server listening on http://localhost:%d", cfg.ServerPort)
	if err := server.Start(cfg.ServerPort, client, store, broadcaster); err != nil {
		log.Fatal(err)
	}
}

package prices

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

const volumeCacheTTL = 24 * time.Hour

// PriceData holds Buff163 and Youpin ask prices for one item.
// All values are in cents. Zero means no price available for that provider.
// Volume is the 24h Steam Market sales count (0 if unavailable).
type PriceData struct {
	BuffAsk   int // buff163 lowest ask, cents
	YoupinAsk int // youpin lowest ask, cents
	Volume    int // 24h Steam Market sales volume
}

// Reference returns the lower of Buff and Youpin, falling back to whichever
// is available. Returns 0 if neither has a price.
func (p PriceData) Reference() int {
	switch {
	case p.BuffAsk > 0 && p.YoupinAsk > 0:
		if p.BuffAsk < p.YoupinAsk {
			return p.BuffAsk
		}
		return p.YoupinAsk
	case p.BuffAsk > 0:
		return p.BuffAsk
	case p.YoupinAsk > 0:
		return p.YoupinAsk
	default:
		return 0
	}
}

// DB is a periodically-refreshed in-memory price store backed by cs2.sh,
// with a SQLite-persisted volume cache sourced from Steam Market.
type DB struct {
	mu           sync.RWMutex
	prices       map[string]PriceData
	client       *cs2shClient
	refreshEvery time.Duration

	volMu    sync.RWMutex
	volCache map[string]int  // in-memory volume cache
	volFetch map[string]bool // in-flight fetch guards
	steam    *steamClient

	sqlDB *sql.DB
}

// NewDB creates a DB that fetches all Buff163 + Youpin prices from cs2.sh
// and refreshes them every refreshEvery interval.
// sqlDB may be nil, in which case volume data is not persisted across restarts.
func NewDB(apiKey string, refreshEvery time.Duration, sqlDB *sql.DB) *DB {
	db := &DB{
		prices:       make(map[string]PriceData),
		client:       newCS2ShClient(apiKey),
		refreshEvery: refreshEvery,
		volCache:     make(map[string]int),
		volFetch:     make(map[string]bool),
		steam:        newSteamClient(),
		sqlDB:        sqlDB,
	}
	if sqlDB != nil {
		if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS volume_cache (
			market_hash_name TEXT PRIMARY KEY,
			volume           INTEGER NOT NULL,
			fetched_at       INTEGER NOT NULL
		)`); err != nil {
			log.Fatalf("prices: create volume_cache table: %v", err)
		}
		if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS prices_cache (
			market_hash_name TEXT PRIMARY KEY,
			buff_ask         INTEGER NOT NULL,
			youpin_ask       INTEGER NOT NULL,
			fetched_at       INTEGER NOT NULL
		)`); err != nil {
			log.Fatalf("prices: create prices_cache table: %v", err)
		}
	}
	return db
}

// Start loads prices from SQLite if the cache is fresh, otherwise fetches
// from cs2.sh. Then starts the hourly background refresh ticker.
// Blocks until prices are available.
func (db *DB) Start() {
	if !db.loadFromCache() {
		if err := db.refresh(); err != nil {
			log.Printf("prices: initial fetch failed: %v", err)
		}
	}
	go db.loop()
}

func (db *DB) loop() {
	ticker := time.NewTicker(db.refreshEvery)
	defer ticker.Stop()
	for range ticker.C {
		if err := db.refresh(); err != nil {
			log.Printf("prices: refresh failed: %v", err)
		}
	}
}

func (db *DB) refresh() error {
	fetched, err := db.client.fetchAll()
	if err != nil {
		return err
	}
	db.mu.Lock()
	db.prices = fetched
	db.mu.Unlock()

	db.saveToCache(fetched)
	log.Printf("prices: fetched %d items from cs2.sh (buff+youpin)", len(fetched))
	return nil
}

// loadFromCache loads prices from SQLite if they are younger than refreshEvery.
// Returns true if prices were loaded and the API call was skipped.
func (db *DB) loadFromCache() bool {
	if db.sqlDB == nil {
		return false
	}
	var maxFetchedAt int64
	if err := db.sqlDB.QueryRow(`SELECT MAX(fetched_at) FROM prices_cache`).Scan(&maxFetchedAt); err != nil || maxFetchedAt == 0 {
		return false
	}
	age := time.Since(time.Unix(maxFetchedAt, 0))
	if age >= db.refreshEvery {
		return false
	}

	rows, err := db.sqlDB.Query(`SELECT market_hash_name, buff_ask, youpin_ask FROM prices_cache`)
	if err != nil {
		return false
	}
	defer rows.Close()

	loaded := make(map[string]PriceData)
	for rows.Next() {
		var name string
		var pd PriceData
		if err := rows.Scan(&name, &pd.BuffAsk, &pd.YoupinAsk); err != nil {
			continue
		}
		loaded[name] = pd
	}
	if len(loaded) == 0 {
		return false
	}

	db.mu.Lock()
	db.prices = loaded
	db.mu.Unlock()
	log.Printf("prices: loaded %d items from cache (age %v, next refresh in %v)", len(loaded), age.Round(time.Second), (db.refreshEvery - age).Round(time.Second))
	return true
}

// saveToCache persists the full price map to SQLite in a single transaction.
func (db *DB) saveToCache(prices map[string]PriceData) {
	if db.sqlDB == nil {
		return
	}
	tx, err := db.sqlDB.Begin()
	if err != nil {
		log.Printf("prices: cache save: %v", err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM prices_cache`); err != nil {
		log.Printf("prices: cache save: %v", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO prices_cache (market_hash_name, buff_ask, youpin_ask, fetched_at) VALUES (?,?,?,?)`)
	if err != nil {
		log.Printf("prices: cache save: %v", err)
		return
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for name, pd := range prices {
		if _, err := stmt.Exec(name, pd.BuffAsk, pd.YoupinAsk, now); err != nil {
			log.Printf("prices: cache save: %v", err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("prices: cache save: %v", err)
	}
}

// Get returns price data for an item by market_hash_name.
// Volume is merged from the in-memory cache (populated lazily from SQLite or Steam Market).
func (db *DB) Get(marketHashName string) (PriceData, bool) {
	db.mu.RLock()
	p, ok := db.prices[marketHashName]
	db.mu.RUnlock()
	if ok && p.Volume == 0 {
		db.volMu.RLock()
		p.Volume = db.volCache[marketHashName]
		db.volMu.RUnlock()
	}
	return p, ok
}

// RequestVolume triggers a background lookup for the item's Steam Market volume
// if one is not already cached or in flight. Safe to call concurrently.
func (db *DB) RequestVolume(marketHashName string) {
	db.volMu.Lock()
	if db.volCache[marketHashName] > 0 || db.volFetch[marketHashName] {
		db.volMu.Unlock()
		return
	}
	db.volFetch[marketHashName] = true
	db.volMu.Unlock()

	go func() {
		vol := db.lookupVolume(marketHashName)
		db.volMu.Lock()
		delete(db.volFetch, marketHashName)
		if vol > 0 {
			db.volCache[marketHashName] = vol
		}
		db.volMu.Unlock()
	}()
}

// lookupVolume checks SQLite first, then falls back to Steam Market.
func (db *DB) lookupVolume(name string) int {
	if db.sqlDB != nil {
		var vol int
		var fetchedAt int64
		err := db.sqlDB.QueryRow(
			`SELECT volume, fetched_at FROM volume_cache WHERE market_hash_name = ?`, name,
		).Scan(&vol, &fetchedAt)
		if err == nil && vol > 0 && time.Since(time.Unix(fetchedAt, 0)) < volumeCacheTTL {
			return vol
		}
	}

	vol := db.steam.fetchVolume(name)
	if vol > 0 && db.sqlDB != nil {
		db.sqlDB.Exec(
			`INSERT OR REPLACE INTO volume_cache (market_hash_name, volume, fetched_at) VALUES (?,?,?)`,
			name, vol, time.Now().Unix(),
		)
	}
	return vol
}

// Len returns the number of items currently in the price DB.
func (db *DB) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.prices)
}

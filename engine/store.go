package engine

import (
	"database/sql"
	"log"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	s := &Store{db: db}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS deals (
		listing_id   TEXT PRIMARY KEY,
		item_name    TEXT NOT NULL,
		condition    TEXT NOT NULL,
		float        REAL NOT NULL,
		paint_seed   INTEGER NOT NULL,
		price        INTEGER NOT NULL,
		ref_price    INTEGER NOT NULL,
		buff_price   INTEGER NOT NULL,
		youpin_price INTEGER NOT NULL,
		discount     REAL NOT NULL,
		sticker_sp   REAL NOT NULL,
		volume       INTEGER NOT NULL DEFAULT 0,
		listing_url  TEXT NOT NULL,
		icon_url     TEXT NOT NULL,
		matched_by   TEXT NOT NULL,
		detected_at  INTEGER NOT NULL
	)`); err != nil {
		log.Fatalf("store: create table: %v", err)
	}
	return s
}

// Add persists the deal. Returns false if the listing_id was already stored.
func (s *Store) Add(deal Deal) bool {
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO deals
		 (listing_id, item_name, condition, float, paint_seed, price, ref_price,
		  buff_price, youpin_price, discount, sticker_sp, volume,
		  listing_url, icon_url, matched_by, detected_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		deal.ListingID, deal.ItemName, deal.Condition, deal.Float, deal.PaintSeed,
		deal.Price, deal.RefPrice, deal.BuffPrice, deal.YoupinPrice,
		deal.Discount, deal.StickerSP, deal.Volume,
		deal.ListingURL, deal.IconURL, deal.MatchedBy, deal.DetectedAt.Unix(),
	)
	if err != nil {
		log.Printf("store: add: %v", err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// Recent returns the n most recently detected deals.
func (s *Store) Recent(n int) []Deal {
	rows, err := s.db.Query(
		`SELECT listing_id, item_name, condition, float, paint_seed, price, ref_price,
		        buff_price, youpin_price, discount, sticker_sp, volume,
		        listing_url, icon_url, matched_by, detected_at
		 FROM deals ORDER BY detected_at DESC LIMIT ?`, n)
	if err != nil {
		log.Printf("store: recent: %v", err)
		return nil
	}
	defer rows.Close()

	var deals []Deal
	for rows.Next() {
		var d Deal
		var ts int64
		if err := rows.Scan(
			&d.ListingID, &d.ItemName, &d.Condition, &d.Float, &d.PaintSeed,
			&d.Price, &d.RefPrice, &d.BuffPrice, &d.YoupinPrice,
			&d.Discount, &d.StickerSP, &d.Volume,
			&d.ListingURL, &d.IconURL, &d.MatchedBy, &ts,
		); err != nil {
			log.Printf("store: scan: %v", err)
			continue
		}
		d.DetectedAt = time.Unix(ts, 0)
		deals = append(deals, d)
	}
	return deals
}

// Baseline returns the detected_at time of the most recent deal in the DB,
// or zero if the table is empty. The poller uses this on startup to skip
// listings that were already processed before the last restart.
func (s *Store) Baseline() time.Time {
	var ts sql.NullInt64
	s.db.QueryRow(`SELECT MAX(detected_at) FROM deals`).Scan(&ts)
	if !ts.Valid {
		return time.Time{}
	}
	return time.Unix(ts.Int64, 0)
}

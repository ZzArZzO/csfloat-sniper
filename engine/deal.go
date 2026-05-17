package engine

import "time"

type Deal struct {
	ListingID   string    `json:"listing_id"`
	ItemName    string    `json:"item_name"`
	Condition   string    `json:"condition"`
	Float       float64   `json:"float"`
	PaintSeed   uint      `json:"paint_seed"`
	Price       int       `json:"price"`        // cents
	RefPrice    int       `json:"ref_price"`    // cents (min of buff/youpin, or csfloat predicted)
	BuffPrice   int       `json:"buff_price"`   // cents (0 if unavailable)
	YoupinPrice int       `json:"youpin_price"` // cents (0 if unavailable)
	Discount    float64   `json:"discount"`     // percentage
	StickerSP   float64   `json:"sticker_sp"`   // sticker total value / price * 100
	Volume      int       `json:"volume"`       // 30-day sales count (0 = unknown)
	ListingURL  string    `json:"listing_url"`
	IconURL     string    `json:"icon_url"`
	MatchedBy   string    `json:"matched_by"` // "price" | "float" | "pattern"
	DetectedAt  time.Time `json:"detected_at"`
}

package engine

import (
	"strings"
	"time"

	csfloat "github.com/Bios-Marcel/csfloat_go"
	"github.com/user/csfloat-sniper/config"
	"github.com/user/csfloat-sniper/prices"
)

type Filter struct {
	cfg *config.Config
	db  *prices.DB
}

func NewFilter(cfg *config.Config, db *prices.DB) *Filter {
	return &Filter{cfg: cfg, db: db}
}

func (f *Filter) Evaluate(listing csfloat.ActiveListing) []Deal {
	refPrice := listing.Reference.PredictedPrice
	var buffPrice, youpinPrice, volume int

	if f.db != nil {
		if pd, ok := f.db.Get(listing.Item.MarketHashName); ok {
			buffPrice = pd.BuffAsk
			youpinPrice = pd.YoupinAsk
			volume = pd.Volume
			if ref := pd.Reference(); ref > 0 {
				refPrice = ref
			}
		}
		if volume == 0 {
			f.db.RequestVolume(listing.Item.MarketHashName)
		}
	}

	stickerVal := stickerTotal(listing.Item.Stickers)
	minStickerCents := int(f.cfg.Filters.MinStickerValue * 100)
	if minStickerCents > 0 && stickerVal < minStickerCents {
		stickerVal = 0
	}

	var discount float64
	if refPrice > 0 && listing.Price > 0 {
		discount = float64(refPrice-listing.Price) / float64(refPrice) * 100
	}

	var stickerSP float64
	if listing.Price > 0 && stickerVal > 0 {
		stickerSP = float64(stickerVal) / float64(listing.Price) * 100
	}

	base := Deal{
		ListingID:   listing.ID,
		ItemName:    listing.Item.MarketHashName,
		Condition:   conditionFromFloat(listing.Item.Float),
		Float:       listing.Item.Float,
		PaintSeed:   listing.Item.PaintSeed,
		Price:       listing.Price,
		RefPrice:    refPrice,
		BuffPrice:   buffPrice,
		YoupinPrice: youpinPrice,
		Discount:    discount,
		StickerSP:   stickerSP,
		Volume:      volume,
		ListingURL:  listing.URL(),
		IconURL:     listing.Item.IconURL,
		DetectedAt:  time.Now(),
	}

	if !f.passesGlobal(listing.Item.MarketHashName, listing.Price, refPrice, volume) {
		return nil
	}

	if f.cfg.PriceSnipe.Enabled && refPrice > 0 && discount >= f.cfg.PriceSnipe.MinDiscountPct {
		d := base
		d.MatchedBy = "price"
		return []Deal{d}
	}

	if f.cfg.FloatSnipe.Enabled {
		for _, rule := range f.cfg.FloatSnipe.Rules {
			if strings.EqualFold(listing.Item.MarketHashName, rule.Item) &&
				listing.Item.Float > 0 && listing.Item.Float <= rule.MaxFloat {
				d := base
				d.MatchedBy = "float"
				return []Deal{d}
			}
		}
	}

	if f.cfg.PatternSnipe.Enabled {
		for _, rule := range f.cfg.PatternSnipe.Rules {
			if strings.EqualFold(listing.Item.MarketHashName, rule.Item) {
				for _, seed := range rule.Seeds {
					if listing.Item.PaintSeed == seed {
						d := base
						d.MatchedBy = "pattern"
						return []Deal{d}
					}
				}
			}
		}
	}

	return nil
}

// passesGlobal returns false if the listing should be skipped regardless of
// which snipe rule matched. All monetary thresholds in config are USD; prices
// here are in cents.
func (f *Filter) passesGlobal(name string, price, refPrice, volume int) bool {
	g := f.cfg.Filters

	if g.MinPrice > 0 && price < int(g.MinPrice*100) {
		return false
	}
	if g.MaxPrice > 0 && price > int(g.MaxPrice*100) {
		return false
	}
	if g.MinSaving > 0 && refPrice-price < int(g.MinSaving*100) {
		return false
	}
	// If a volume gate is set and volume is unknown (0), block until cache warms.
	if g.MinVolume > 0 && volume < g.MinVolume {
		return false
	}
	if g.ExcludeStatTrak && strings.Contains(name, "StatTrak™") {
		return false
	}
	if g.ExcludeSouvenir && strings.Contains(name, "Souvenir") {
		return false
	}
	return true
}

func stickerTotal(stickers []csfloat.Sticker) int {
	total := 0
	for _, s := range stickers {
		total += int(s.Reference.Price)
	}
	return total
}

func conditionFromFloat(f float64) string {
	switch {
	case f <= 0.07:
		return "Factory New"
	case f <= 0.15:
		return "Minimal Wear"
	case f <= 0.38:
		return "Field-Tested"
	case f <= 0.45:
		return "Well-Worn"
	default:
		return "Battle-Scarred"
	}
}

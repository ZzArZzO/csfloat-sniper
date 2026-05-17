package engine

import (
	"log"
	"math/rand/v2"
	"time"

	"github.com/user/csfloat-sniper/api"
	"github.com/user/csfloat-sniper/config"
)

type Poller struct {
	client      *api.Client
	filter      *Filter
	store       *Store
	broadcaster *Broadcaster
	notifiers   []Notifier
	interval    time.Duration
	jitter      time.Duration
	lastSeen    time.Time
}

func NewPoller(client *api.Client, filter *Filter, store *Store, broadcaster *Broadcaster, cfg *config.Config, notifiers ...Notifier) *Poller {
	p := &Poller{
		client:      client,
		filter:      filter,
		store:       store,
		broadcaster: broadcaster,
		notifiers:   notifiers,
		interval:    cfg.PollInterval.Duration,
		jitter:      cfg.PollJitter.Duration,
	}
	if baseline := store.Baseline(); !baseline.IsZero() {
		p.lastSeen = baseline
		log.Printf("poller: resuming from DB baseline %v", baseline.Format("2006-01-02 15:04:05"))
	}
	return p
}

func (p *Poller) Run() {
	p.poll()
	for {
		sleep := p.interval
		if p.jitter > 0 {
			// Add a random offset in [-jitter, +jitter].
			sleep += time.Duration(rand.Int64N(int64(2*p.jitter+1))) - p.jitter
		}
		time.Sleep(sleep)
		p.poll()
	}
}

func (p *Poller) poll() {
	listings, err := p.client.FetchNewest()
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	if p.lastSeen.IsZero() {
		// First poll: establish baseline, don't fire alerts for stale listings.
		for _, l := range listings {
			if l.CreatedAt.After(p.lastSeen) {
				p.lastSeen = l.CreatedAt
			}
		}
		log.Printf("baseline set: %d listings, newest at %v", len(listings), p.lastSeen)
		return
	}

	var newest time.Time
	for _, listing := range listings {
		if !listing.CreatedAt.After(p.lastSeen) {
			continue
		}
		if listing.CreatedAt.After(newest) {
			newest = listing.CreatedAt
		}
		for _, deal := range p.filter.Evaluate(listing) {
			if p.store.Add(deal) {
				p.broadcaster.Publish(deal)
				for _, n := range p.notifiers {
					n.Notify(deal)
				}
				log.Printf("deal found: %s matched by %s (%.1f%% off)", deal.ItemName, deal.MatchedBy, deal.Discount)
			}
		}
	}

	if !newest.IsZero() {
		p.lastSeen = newest
	}
}

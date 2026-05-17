package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/user/csfloat-sniper/api"
	"github.com/user/csfloat-sniper/engine"
)

func dealsHandler(store *engine.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(store.Recent(50))
	}
}

func buyHandler(client *api.Client, store *engine.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var price int
		for _, d := range store.Recent(maxDeals) {
			if d.ListingID == id {
				price = d.Price
				break
			}
		}
		if price == 0 {
			http.Error(w, "listing not found", http.StatusNotFound)
			return
		}

		if err := client.BuyListing(id, price); err != nil {
			http.Error(w, fmt.Sprintf("buy failed: %v", err), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func sseHandler(broadcaster *engine.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		ch := broadcaster.Subscribe()
		defer broadcaster.Unsubscribe(ch)

		// Flush headers immediately so the browser fires onopen.
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case deal, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(deal)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-keepalive.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

package server

import (
	"fmt"
	"net/http"

	"github.com/user/csfloat-sniper/api"
	"github.com/user/csfloat-sniper/engine"
)

const maxDeals = 200

func Start(port int, client *api.Client, store *engine.Store, broadcaster *engine.Broadcaster) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	mux.HandleFunc("GET /static/app.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/app.js")
	})
	mux.HandleFunc("GET /events", sseHandler(broadcaster))
	mux.HandleFunc("GET /deals", dealsHandler(store))
	mux.HandleFunc("POST /buy/{id}", buyHandler(client, store))

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, mux)
}

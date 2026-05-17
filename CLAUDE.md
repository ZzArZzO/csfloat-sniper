# CSFloat Sniper

A Go-based sniping dashboard for CSFloat that monitors new listings in real time, flags underpriced deals, and lets the user buy with a single click.

## How to Run

```bash
GOEXPERIMENT=jsonv2 go run main.go
```

Then open `http://localhost:8080`.

> `GOEXPERIMENT=jsonv2` is required because the `csfloat_go` dependency uses Go's experimental JSON v2 package.

## Architecture

Two flows working together:

```
Request Flow:   Poll Cycle → Key Rotator → HTTP Client → CSFloat API
Data Pipeline:  CSFloat API → Dedup → Price Lookup → Analyze → Notify
```

### Components

| Component | File(s) | Status |
|---|---|---|
| Poll Cycle | `engine/poller.go` | Done — jittered interval, timestamp dedup |
| Key Rotator | `api/client.go` | Done — round-robin across poll keys |
| HTTP Client | `api/client.go` | Partial — no proxy support yet |
| CSFloat API | `api/client.go` | Done — `GET /listings?sort_by=most_recent` |
| Dedup | `engine/store.go` | Done — in-memory (intentional, no SQLite) |
| Price Lookup | `prices/` + `engine/filter.go` | Done — cs2.sh Buff+Youpin prices; falls back to `predicted_price` if key not set |
| Analyze | `engine/filter.go` | Done — discount % against Buff/Youpin reference, sticker SP% |
| Notify | `engine/broadcaster.go`, `server/`, `notify/discord.go` | Done — SSE web UI + Discord webhook |

## Config (`config.yaml`)

```yaml
main_api_key: "..."        # Your main account — used ONLY for buying
poll_api_keys:             # Secondary accounts — used for polling (round-robin)
  - "..."
poll_interval: 10s         # Effective poll interval across all keys
poll_jitter: 2s            # Each tick is interval ± up to 2s (so 8s–12s)
server_port: 8080

cs2cap_api_key: "..."      # cs2.sh developer key — enables Buff+Youpin prices
price_refresh_interval: 1h # How often to re-fetch the full price DB (1 request)

discord_webhook_url: "https://discord.com/api/webhooks/..." # Leave empty to disable

price_snipe:
  enabled: true
  min_discount_pct: 20.0   # % below reference price

float_snipe:
  enabled: true
  rules:
    - item: "AK-47 | Redline (Field-Tested)"
      max_float: 0.15

pattern_snipe:
  enabled: true
  rules:
    - item: "AK-47 | Case Hardened (Field-Tested)"
      seeds: [661, 321, 179]
```

### Key rotation math

Each key has a ~200 req/hour limit. With N poll keys at interval T:
- Requests per key per hour = 3600 / (N × T)
- Must be ≤ 200

| Poll keys | Safe interval | Effective poll |
|---|---|---|
| 1 | 20s | 20s |
| 2 | 10s | 10s |
| 3 | 7s | 7s |
| 4 | 5s | 5s |

## Project Structure

```
CSFloat2/
├── main.go                  # Entry point
├── config.yaml              # Runtime config (API keys, rules, intervals)
├── config/config.go         # Config structs + YAML loading
├── api/client.go            # CSFloat API wrapper + key rotation pool
├── engine/
│   ├── deal.go              # Deal struct (shared type)
│   ├── notifier.go          # Notifier interface (implemented by notify pkg)
│   ├── filter.go            # Price / float / pattern snipe rules
│   ├── store.go             # Thread-safe in-memory deal ring buffer (cap 200)
│   ├── broadcaster.go       # SSE pub-sub fan-out to browser clients
│   └── poller.go            # Polling goroutine with timestamp-based dedup
├── notify/
│   └── discord.go           # Discord webhook notifier (rich embeds)
├── prices/
│   ├── db.go                # Periodic full-refresh price store (sync.RWMutex map)
│   └── cs2sh.go             # cs2.sh client — fetches all Buff+Youpin prices in one request
├── server/
│   ├── server.go            # HTTP mux
│   └── handlers.go          # /deals, /events (SSE), /buy/{id}
└── web/
    ├── index.html           # Three-column dashboard (price / float / pattern)
    └── app.js               # Vanilla JS — SSE listener, buy button, age refresh
```

## Key Decisions

- **No auto-buy** — user reviews deals and clicks Buy manually. The buy call goes through `main_api_key` exclusively, never a secondary key.
- **Secondary keys for polling only** — secondary accounts never trade anything, only read public listing data. Main account is never at risk.
- **Timestamp-based dedup in poller** — on first poll, establish a baseline (newest listing time). Only process listings newer than baseline going forward. Avoids flooding with historical deals on startup.
- **Jitter on poll interval** — each sleep is `interval ± jitter` (random). Configured via `poll_jitter` in config.yaml. Makes polling look like human browser traffic rather than a fixed-cadence bot.
- **SQLite (`sniper.db`)** — deals and volume cache are persisted via `modernc.org/sqlite` (pure Go, no CGO). Deal history survives restarts. Poller loads its `lastSeen` baseline from `MAX(detected_at)` in the deals table — no duplicate burst on restart. Volume cache has a 24h TTL.
- **cs2.sh for reference prices** — single request fetches all ~40K items (Buff163 + Youpin asks). Refreshed every hour. Falls back to CSFloat `predicted_price` if key is not set.
- **`GOEXPERIMENT=jsonv2`** — required at build/run time due to `csfloat_go` dependency. Not a runtime flag.

## Next Steps

### 1. cs2.sh API Key (needed for real prices)
Sign up at cs2.sh, get the free 7-day developer key, paste into `config.yaml`:
```yaml
cs2cap_api_key: "YOUR_CS2SH_KEY"
```
On startup you should see: `prices: loaded ~40000 items from cs2.sh (buff+youpin)`

## CSFloat API Notes

- Base URL: `https://csfloat.com/api/v1/`
- Auth: `Authorization: <api-key>` header
- Rate limit: **200 requests/hour per key** (headers: `X-Ratelimit-Remaining`, `X-Ratelimit-Reset`)
- Buy endpoint: `POST /listings/buy` with `{ contract_ids: [...], total_price: N }`
- Listings endpoint: `GET /listings?sort_by=most_recent&type=buy_now&limit=40`
- All prices are in **cents** (divide by 100 for dollars)
- `predicted_price` = CSFloat's float-adjusted reference price (not Buff163)

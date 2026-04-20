# flagd

A feature flag server for your homelab. Single Go binary, Redis-backed, zero config.

Deploy a flag, check it from your app, toggle it from a dashboard - no redeploy required.

```bash
# your app checks a flag
curl https://flagd.yourdomain.com/flags/dark-mode
# {"name":"dark-mode","enabled":true,...}
```

```go
// or use the Go SDK
client := sdk.New("https://flagd.yourdomain.com")
if client.IsEnabled(ctx, "dark-mode") {
    // ship it
}
```

## Why

Deploying code and releasing features are two different things. Feature flags let you merge and deploy freely, then release to users by flipping a switch. No redeployment, no downtime, instant rollback.

## How it works

```
your app  →  GET /flags/{name}  →  {"enabled": true}
              (no auth required, fast path for reads)

you       →  PUT/PATCH/DELETE   →  requires Bearer token
              (write routes protected by ADMIN_KEY)
```

Each flag is a Redis hash. A Redis set tracks all flag names so listing is O(1) without scanning the keyspace. Toggle is a Lua script so the read-modify-write is atomic - two simultaneous toggles can't race.

```
flag:dark-mode  →  hash { enabled, description, created_at, updated_at }
flags           →  set  { "dark-mode", "new-checkout", ... }
```

## Deploy to k3s with deployit

```bash
deployit deploy . --host flagd.yourdomain.com --with redis
deployit secrets flagd ADMIN_KEY=$(openssl rand -hex 32)
```

Redis is provisioned automatically, connection string injected as `REDIS_URL`.

## Run locally

```bash
docker run -d -p 6379:6379 redis:7-alpine
ADMIN_KEY=secret go run .
```

Or with Docker Compose:

```yaml
services:
  flagd:
    build: .
    ports: ["8080:8080"]
    environment:
      REDIS_URL: redis://redis:6379
      ADMIN_KEY: your-secret-key
    depends_on: [redis]
  redis:
    image: redis:7-alpine
    volumes: [redis-data:/data]
    command: redis-server --appendonly yes

volumes:
  redis-data:
```

## API

Read routes are public. Write routes require `Authorization: Bearer <ADMIN_KEY>`.

```bash
# Create or update a flag
curl -X PUT https://flagd.yourdomain.com/flags/dark-mode \
  -H 'Authorization: Bearer <key>' \
  -H 'Content-Type: application/json' \
  -d '{"enabled": false, "description": "Dark mode UI"}'

# Check a flag - no auth needed, this is what your apps call
curl https://flagd.yourdomain.com/flags/dark-mode
# {"name":"dark-mode","enabled":false,"description":"Dark mode UI","created_at":"...","updated_at":"..."}

# List all flags
curl https://flagd.yourdomain.com/flags

# Toggle a flag atomically
curl -X PATCH https://flagd.yourdomain.com/flags/dark-mode/toggle \
  -H 'Authorization: Bearer <key>'

# Delete a flag
curl -X DELETE https://flagd.yourdomain.com/flags/dark-mode \
  -H 'Authorization: Bearer <key>'
```

## Dashboard

Visit `/ui` in your browser. Paste your admin key into the input - it's saved to sessionStorage so you only enter it once per session. Create, toggle, and delete flags without curl.

## Go SDK

```go
import "github.com/gappylul/flagd/sdk"

client := sdk.New("https://flagd.yourdomain.com")

// Safe - returns false on any error, never panics
// A dead flag server won't break your app
if client.IsEnabled(ctx, "dark-mode") {
    // feature is on
}

// Get the full flag object
flag, err := client.Get(ctx, "dark-mode")

// List all flags
flags, err := client.List(ctx)

// Custom timeout (default: 2s)
client := sdk.New("https://flagd.yourdomain.com", sdk.WithTimeout(500*time.Millisecond))
```

## Config

| Env var      | Default                | Description                                                                              |
|--------------|------------------------|------------------------------------------------------------------------------------------|
| `REDIS_URL`  | redis://localhost:6379 | Redis connection URL                                                                     |
| `ADMIN_KEY`  | (none)                 | Bearer token for writes. If unset, write routes are unprotected and a warning is logged. |
| `FLAGD_ADDR` | :8080                  | Listen address                                                                           |

## Project structure

```
main.go                          entry point, wires Redis + HTTP server
internal/
  api/
    api.go                       JSON API routes and handlers
    ui.go                        HTMX dashboard routes (embeds templates + CSS)
    middleware.go                Bearer auth middleware for write routes
    templates/
      static/
        style.css                dashboard styles
      page.html                  dashboard page
      flag_row.html              HTMX partial - single flag row
  store/
    store.go                     all Redis operations (List, Get, Upsert, Toggle, Delete)
sdk/
  sdk.go                         importable Go client
```

## Design decisions

**Redis over a relational DB** - flags are read on every request from every service. Redis is in-memory and fast. The data model is simple enough that a hash per flag is all you need.

**Lua script for Toggle** - `HGET` → flip → `HSET` is three operations. Without atomicity, two simultaneous toggles could both read `false` and both write `true`. The Lua script runs atomically on the Redis server, making this a non-issue.

**`IsEnabled` never returns an error** - the SDK swallows errors and returns `false`. If flagd goes down, your features turn off gracefully instead of your app crashing. Uptime of the flag server is not on the critical path.

**`hx-headers` on `<body>` for auth** - the HTMX dashboard needs to send the Bearer token on every mutating request. Setting `document.body.setAttribute('hx-headers', ...)` when the key is entered is more reliable than the `htmx:configRequest` event, since HTMX inherits and merges `hx-headers` from parent elements automatically.
# URL Shortener

A simple URL shortener API built with Go, using SQLite for storage.

## Project Structure

```
├── main.go                        # Server + routing + graceful shutdown
├── main_test.go                   # Config loading tests
├── store/
│   ├── types.go                   # URL, ErrConflict, URLStore interface
│   ├── open.go                    # Store concrete type + Open() factory
│   ├── migrate.go                 # Embedded migration runner
│   └── sqlite/
│       ├── sqlite.go              # SQLite connection + IsConstraintError
│       └── migrations/
│           └── 001_init.sql       # Initial schema
├── shortener/shortener.go         # Random 6-char code generator
├── auth/auth.go                   # Bearer token middleware
└── handler/handler.go             # HTTP handlers
```

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/shorten` | Bearer | `{"url":"https://..."}` → 201 + code |
| `GET` | `/{code}` | None | → 301 redirect |
| `GET` | `/health` | None | → 200 `{"status":"ok"}` |

## Run

Set the required environment variable and start the server:

```bash
SHORTENER_TOKEN=mysecret go run .
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SHORTENER_TOKEN` | Yes | — | Bearer token for the `/shorten` endpoint |
| `DB_DRIVER` | No | `sqlite` | Database driver (`sqlite`) |
| `DB_DSN` | No | `shortener.db` | Data source name (`:memory:`, `shortener.db`, `postgres://...`) |
| `PORT` | No | `8080` | Server listen port |
| `SHUTDOWN_TIMEOUT` | No | `5s` | Graceful shutdown timeout (Go duration) |

Example with custom config:

```bash
SHORTENER_TOKEN=mysecret DB_DSN=:memory: PORT=9090 SHUTDOWN_TIMEOUT=10s go run .
```

## Usage

Create a short URL:

```bash
curl -X POST localhost:8080/shorten \
  -H "Authorization: Bearer mysecret" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

Resolve a short URL (follows redirect):

```bash
curl -L localhost:8080/<code>
```

## Tests

```bash
CGO_ENABLED=1 go test ./...
```

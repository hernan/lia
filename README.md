# URL Shortener

A simple URL shortener API built with Go, using SQLite for storage.

## Project Structure

```
├── main.go                  # Server + routing + graceful shutdown
├── store/store.go           # SQLite CRUD (Create, GetByCode)
├── shortener/shortener.go   # Random 6-char code generator
├── auth/auth.go             # Bearer token middleware
└── handler/handler.go       # HTTP handlers (Shorten, Resolve, Health)
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

Optionally set `DB_PATH` to choose where SQLite stores data (defaults to `shortener.db`):

```bash
SHORTENER_TOKEN=mysecret DB_PATH=data.db go run .
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

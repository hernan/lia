# URL Shortener

A simple URL shortener API built with Go, using SQLite for storage. Includes a server-rendered admin panel for managing shortened URLs.

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
├── handler/handler.go             # HTTP handlers (API)
├── admin/
│   ├── admin.go                   # Admin panel handlers
│   └── templates/                 # Embedded HTML templates
│       ├── login.html
│       ├── dashboard.html
│       └── edit.html
└── internal/
    ├── httputil/                  # Shared JSON response helpers
    └── session/                   # HMAC session + CSRF manager
```

## Endpoints

### API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/shorten` | Bearer | `{"url":"https://..."}` → 201 + code |
| `GET` | `/{code}` | None | → 301 redirect |
| `GET` | `/health` | None | → 200 `{"status":"ok"}` |

### Admin Panel

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/admin/login` | None | Login page |
| `POST` | `/admin/login` | None | Authenticate |
| `POST` | `/admin/logout` | Session | Destroy session |
| `GET` | `/admin` | Session | Dashboard — list/search URLs |
| `POST` | `/admin` | Session | Create a new short URL |
| `GET` | `/admin/urls/{id}/edit` | Session | Edit URL form |
| `POST` | `/admin/urls/{id}/edit` | Session | Update URL |
| `POST` | `/admin/urls/{id}/delete` | Session | Delete URL |

Admin routes are protected by cookie-based session auth with CSRF protection. The admin panel is only available when `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set.

## Run

Set the required environment variable and start the server:

```bash
SHORTENER_TOKEN=mysecret go run .
```

To enable the admin panel, also set:

```bash
SHORTENER_TOKEN=mysecret ADMIN_USERNAME=admin ADMIN_PASSWORD=changeme go run .
```

Then visit http://localhost:8080/admin/login.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SHORTENER_TOKEN` | Yes | — | Bearer token for the `/shorten` endpoint |
| `ADMIN_USERNAME` | No | — | Admin panel username (enables admin panel) |
| `ADMIN_PASSWORD` | No | — | Admin panel password |
| `DB_DRIVER` | No | `sqlite` | Database driver (`sqlite`) |
| `DB_DSN` | No | `shortener.db` | Data source name (`:memory:`, `shortener.db`, `postgres://...`) |
| `PORT` | No | `8080` | Server listen port |
| `SHUTDOWN_TIMEOUT` | No | `5s` | Graceful shutdown timeout (Go duration) |

The admin panel is disabled unless both `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set.

Example with custom config:

```bash
SHORTENER_TOKEN=mysecret ADMIN_USERNAME=admin ADMIN_PASSWORD=changeme DB_DSN=:memory: PORT=8080 SHUTDOWN_TIMEOUT=10s go run .
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

## Adding a New Database Driver

Adding a new driver (e.g. Postgres) requires changes in four files plus
a new package. Using `postgres` as an example:

### 1. Create the driver package

```
store/postgres/postgres.go
store/postgres/migrations/001_init.sql
```

`store/postgres/postgres.go` must export:

```go
// New opens a Postgres database and returns the connection.
func New(dsn string) (*sql.DB, error) { ... }

// IsConstraintError reports whether err is a Postgres constraint violation.
func IsConstraintError(err error) bool { ... }
```

`New` should configure connection pool settings appropriate for the driver.
`IsConstraintError` should check the driver-specific error type (e.g.
`pgconn.PgError` with code `23505` for Postgres unique violations).

The `migrations/001_init.sql` file contains the initial schema using
the driver's SQL dialect.

### 2. Register migrations in `store/migrate.go`

Add an embed directive and two case branches:

```go
//go:embed all:postgres/migrations
var postgresMigrations embed.FS
```

In `migrationFiles`:

```go
case "postgres":
    fs = postgresMigrations
```

In `readMigration`:

```go
case "postgres":
    fs = postgresMigrations
```

### 3. Register the driver in `store/open.go`

Import the new package:

```go
"urlshortener/store/postgres"
```

In `Open`:

```go
case "postgres":
    db, err = postgres.New(dsn)
```

In `isConstraintError`:

```go
case "postgres":
    return postgres.IsConstraintError(err)
```

### 4. Use it

```bash
DB_DRIVER=postgres DB_DSN="postgres://user:pass@localhost:5432/shortener" go run .
```

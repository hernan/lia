# Admin Panel — Implementation Plan

## Overview

Add a server-rendered admin panel with cookie-based session auth (separate from the existing bearer token API). Single admin user defined via env vars. Go `html/template` for rendering.

## New Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ADMIN_USERNAME` | Yes | Admin login username |
| `ADMIN_PASSWORD` | Yes | Admin login password |

## Files to Create/Modify

```
├── store/types.go                         # Add List, Search, Update, Delete, GetByID to interface
├── store/open.go                          # Implement new methods
├── store/open_test.go                     # Tests for new methods
├── internal/session/session.go            # HMAC-signed cookie session manager + CSRF
├── internal/session/session_test.go       # Session + CSRF tests
├── admin/admin.go                         # Handlers, templates, auth middleware
├── admin/admin_test.go                    # Admin handler tests
├── admin/templates/                       # Embedded via embed.FS
│   ├── login.html
│   ├── dashboard.html
│   └── edit.html
└── main.go                               # Wire admin routes, new env vars
```

---

## Phase 1: Store — New CRUD Methods

Add to `store/types.go` interface and implement in `store/open.go`:

| Method | SQL | Notes |
|--------|-----|-------|
| `List() ([]*URL, error)` | `SELECT ... ORDER BY created_at DESC` | All URLs |
| `Search(query string) ([]*URL, error)` | `WHERE original_url LIKE ?` | `%query%` match |
| `GetByID(id int64) (*URL, error)` | `WHERE id = ?` | Edit form pre-fill |
| `Update(id int64, originalURL string) error` | `UPDATE urls SET original_url = ? WHERE id = ?` | Edit |
| `Delete(id int64) error` | `DELETE FROM urls WHERE id = ?` | Remove |

No migration needed. Tests in `store/open_test.go` using `:memory:` SQLite.

---

## Phase 2: Session + CSRF

Create `internal/session/session.go`:

### Session

- `Manager` struct with `secret []byte`
- `New(secret []byte) *Manager`
- `Create(username string) *http.Cookie` — cookie value: `username|HMAC(username)`
- `Validate(r *http.Request) (string, error)` — parse + verify HMAC
- `Destroy(w http.ResponseWriter)` — expired cookie

### CSRF (double-submit cookie pattern)

- `GenerateToken() string` — random hex token via `crypto/rand`
- `SetCSRFCookie(w, token)` — sets non-httponly `csrf_token` cookie
- `TokenFromCookie(r *http.Request) string` — reads current CSRF token
- `Verify(r *http.Request) bool` — compares form value `csrf_token` against cookie

Tests in `internal/session/session_test.go`.

---

## Phase 3: Admin Package

`admin/admin.go` — consumer-side interface + handlers:

```go
type URLStore interface {
    List() ([]*store.URL, error)
    Search(query string) ([]*store.URL, error)
    GetByID(id int64) (*store.URL, error)
    Create(originalURL, code string) (*store.URL, error)
    Update(id int64, originalURL string) error
    Delete(id int64) error
}
```

### Routes

| Method | Path | Handler | Auth | CSRF | Description |
|--------|------|---------|------|------|-------------|
| `GET` | `/admin/login` | `Login` | No | Yes | Render login form |
| `POST` | `/admin/login` | `Login` | No | Yes | Validate creds, set session |
| `POST` | `/admin/logout` | `Logout` | Yes | Yes | Destroy session |
| `GET` | `/admin` | `Dashboard` | Yes | — | List/search URLs |
| `POST` | `/admin` | `CreateURL` | Yes | Yes | Create short URL |
| `GET` | `/admin/urls/{id}/edit` | `EditURL` | Yes | — | Render edit form |
| `POST` | `/admin/urls/{id}/edit` | `UpdateURL` | Yes | Yes | Update URL |
| `POST` | `/admin/urls/{id}/delete` | `DeleteURL` | Yes | Yes | Delete URL |

### Middleware

- `requireAuth` — validates session, redirects to `/admin/login` if invalid
- `requireCSRF` — validates form token against cookie, returns 403 if mismatch
- Login page bypasses `requireAuth` but still gets CSRF protection

### Templates (embedded via `//go:embed templates/*.html`)

- **`login.html`** — username/password form + hidden CSRF token
- **`dashboard.html`** — search bar, create form, URL table with edit/delete buttons + CSRF tokens
- **`edit.html`** — edit form with hidden CSRF token

Tests in `admin/admin_test.go` with mock store.

---

## Phase 4: Wire Into `main.go`

1. Add `adminUsername`, `adminPassword` to `config` struct + `loadConfig()`
2. Derive session secret: `sha256(SHORTENER_TOKEN)`
3. Create `session.Manager` and `admin.New(...)`
4. Register admin routes on the mux (before the `/{code}` catch-all)

---

## Execution Order

1. Store CRUD methods + tests
2. Session + CSRF manager + tests
3. Admin templates (login, dashboard, edit)
4. Admin handlers + tests
5. Wire into `main.go` + config
6. Run `CGO_ENABLED=1 go test ./...` and `go vet ./...` to verify

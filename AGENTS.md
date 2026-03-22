# AGENTS.md

## Project overview

A Go URL shortener API using SQLite. The module is `urlshortener` (Go 1.26.1).
The only external dependency is `github.com/mattn/go-sqlite3`.

```
├── main.go                  # Entry point, routing, graceful shutdown
├── main_test.go             # Config loading tests
├── store/store.go           # SQLite CRUD operations
├── shortener/shortener.go   # Random 6-char code generator
├── auth/auth.go             # Bearer token middleware
└── handler/handler.go       # HTTP handlers
```

## Build and test commands

CGO is required for the sqlite3 driver. Always prefix commands with `CGO_ENABLED=1`.

```bash
CGO_ENABLED=1 go build ./...                              # build all packages
CGO_ENABLED=1 go test ./...                               # run all tests
CGO_ENABLED=1 go test ./... -v                            # verbose output
CGO_ENABLED=1 go test ./store/ -v                         # single package
CGO_ENABLED=1 go test ./handler/ -run TestHealth -v       # single test by name
CGO_ENABLED=1 go test ./handler/ -run TestHealthUnhealthy # partial name match
go mod tidy                                               # update go.sum after imports change
```

There is no linter configured. Run `go vet ./...` for basic static analysis.

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SHORTENER_TOKEN` | Yes | — | Bearer token for the `/shorten` endpoint |
| `DB_PATH` | No | `shortener.db` | Path to SQLite database file |
| `PORT` | No | `8080` | Server listen port |
| `SHUTDOWN_TIMEOUT` | No | `5s` | Graceful shutdown timeout (Go duration) |

## Code style

### Imports

Group imports with stdlib first, then a blank line, then third-party, then a
blank line, then project imports. This is enforced by `goimports`.

```go
import (
	"encoding/json"
	"net/http"

	"urlshortener/shortener"
	"urlshortener/store"
)
```

### Naming

- Packages: short, lowercase, single word (`store`, `auth`, `handler`).
- Constructors: `New(...)` returning `(*Type, error)` or `*Type`.
- Exported types and methods use PascalCase. Unexported use camelCase.
- Acronyms stay uppercase: `URL`, `ID`, `HTTP`.
- Interfaces defined by the consumer package: `handler` defines `URLStore`
  rather than depending on the concrete `*store.Store`.

### Error handling

Use the standard `if err != nil` pattern. Return errors up the call stack;
do not log in library packages. In `main.go`, use `log.Fatal` for startup
errors. HTTP handlers write errors as JSON via `httputil.WriteError`.

```go
if err != nil {
	return nil, err
}
```

### JSON responses

Shared helpers in `internal/httputil` encode responses:

```go
httputil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
	"code": created.Code,
	"url":  created.OriginalURL,
})
```

`httputil.WriteError` wraps the common error pattern:

```go
httputil.WriteError(w, http.StatusBadRequest, "url is required")
```

### Structs

Keep fields unexported. Use constructors (`New`) to build instances rather
than exporting struct fields.

> **Exception:** pure data-transfer types that carry no behaviour (e.g.
> `store.URL`) may use exported fields. The "unexported fields + constructor"
> rule applies to types that own significant logic or internal state.

## Testing conventions

- Test files live in the same package (white-box testing).
- Test function names: `Test<Thing>` or `Test<Thing><Condition>` (e.g.
  `TestHealthUnhealthy`).
- Use `t.Helper()` in test helper functions.
- Use `t.Cleanup()` for resource cleanup instead of `defer`.
- For database tests, use in-memory SQLite: `store.New(":memory:")`.
- For HTTP tests, use `net/http/httptest` (`NewServer`, `NewRecorder`).
- For dependency injection in tests, define interfaces in the consumer
  package and provide mock implementations in the `_test.go` file.
- No external test frameworks — use the standard `testing` package only.
- Use `t.Fatal` for setup errors, `t.Errorf` for assertion failures.
- Use `t.Setenv()` to set environment variables in tests (automatically restored).

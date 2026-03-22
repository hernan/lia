# General Cleanup — Implementation Plan

This document describes the 9 tasks that came out of a general code-quality and
maintainability review of the `urlshortener` project. Tasks are ordered so that
bug fixes come first, architectural changes come second, and DRY / test / docs
polish come last. Every task maps 1-to-1 to a single commit and all tests must
be green before that commit is made.

---

## Task 1 — Fix `store.Create` to own the `CreatedAt` timestamp

**File:** `store/store.go`

**Problem:** `store.Create` inserted a row without supplying `created_at`,
letting SQLite set it via `DEFAULT CURRENT_TIMESTAMP`. It then returned
`time.Now().UTC()` as the `CreatedAt` field of the resulting `URL` struct.
Because the two clocks were independent calls, the returned value could drift
from what was actually stored in the database. `TestCreateCreatedAtMatchesDB`
worked around this with a one-second tolerance — a clear sign the abstraction
was leaking.

**Change:** Capture `time.Now().UTC().Truncate(time.Second)` once before the
`INSERT` and pass it explicitly as the `created_at` column value. Assign the
same variable to `URL.CreatedAt` so both sides share the same instant.

**Test:** Updated `TestCreateCreatedAtMatchesDB` to assert exact equality
between the returned `CreatedAt` and the value read back from the database via
`GetByCode`. No tolerance is needed once both sides share the same `now`.

**Commit message:** `fix(store): own CreatedAt timestamp in Create to match DB row exactly`

---

## Task 2 — Add missing `Ping` tests to `store`

**File:** `store/store_test.go`

**Problem:** `Store.Ping()` is part of the public API and is the backbone of
the health-check endpoint, but it had no unit test in the store package.

**Change:** Added two test cases:

- `TestPingHealthy` — calls `Ping` on a freshly opened in-memory store and
  asserts `nil` is returned.
- `TestPingAfterClose` — closes the store, then calls `Ping` and asserts a
  non-nil error is returned.

**Test:** The two new test functions are themselves the deliverable.

**Commit message:** `test(store): add Ping tests for healthy and closed DB`

---

## Task 3 — Configure the connection pool before the first query

**File:** `store/store.go`

**Problem:** `store.New` called `db.Exec` (the table-creation query) before
calling `db.SetMaxOpenConns`, `db.SetMaxIdleConns`, and
`db.SetConnMaxLifetime`. The pool settings therefore did not govern that first
connection. While SQLite with `max_open_conns=1` makes this a non-issue at
runtime, the code order misrepresented the intent and was confusing to readers.

**Change:** Moved the three `db.Set*` calls to immediately after `sql.Open`
and before any `db.Exec`.

**Test:** No new test needed. Existing store tests validate correct behaviour
after construction. Verified by `CGO_ENABLED=1 go test ./store/ -v` passing
without regressions.

**Commit message:** `fix(store): configure connection pool before first query`

---

## Task 4 — Move `isConstraintError` to `store`; expose `ErrConflict` sentinel

**Files:** `store/store.go`, `handler/handler.go`, `handler/handler_test.go`

**Problem:** `handler/handler.go` imported `github.com/mattn/go-sqlite3`
directly to inspect raw driver errors via a local `isConstraintError` helper.
This coupled the HTTP handler layer to the concrete SQLite driver — a violation
of the dependency direction. Any future swap of the storage backend (e.g.,
PostgreSQL) would have required changes inside the handler.

**Change:**

1. In `store/store.go`: declared `var ErrConflict = errors.New("code already
   exists")` and moved `isConstraintError` (private) here. `Create` now detects
   constraint violations and returns `fmt.Errorf("%w", ErrConflict)` instead of
   the raw driver error.
2. In `handler/handler.go`: removed the `github.com/mattn/go-sqlite3` import,
   deleted the local `isConstraintError` function, and replaced the collision
   check with `errors.Is(err, store.ErrConflict)`.
3. In `handler/handler_test.go`: updated `collisionMockStore.Create` to return
   `store.ErrConflict` instead of constructing a `sqlite3.Error` struct.

**Test:** Added `TestCreateDuplicateCodeReturnsErrConflict` to
`store/store_test.go`: inserts a row, then attempts a second insert with the
same code and asserts `errors.Is(err, store.ErrConflict)`.

**Commit message:** `refactor(store): expose ErrConflict sentinel and remove sqlite3 dep from handler`

---

## Task 5 — Replace magic number in `shortener.Generate`

**File:** `shortener/shortener.go`

**Problem:** A previous refactor replaced the `math/big`-based generator with a
faster base32 modular approach (`b[i]%32`). The bare literal `32` was a magic
number that would silently produce incorrect output if `Charset` were ever
changed to a different length.

**Change:** Declared a compile-time constant `charsetLen = byte(len(Charset))`
alongside the existing `Charset` and `Length` constants. Replaced `b[i]%32`
with `b[i]%charsetLen` so the modular reduction is always derived from the
actual charset length.

**Test:** No new test needed. Existing `TestGenerateLength` and
`TestGenerateCharset` confirm correct behaviour after the change. A future
charset change will now automatically keep the reduction in sync.

**Commit message:** `perf(shortener): replace magic number 32 with named charsetLen constant`

---

## Task 6 — Extract `WriteJSON` / `WriteError` into `internal/httputil`

**Files:** `auth/auth.go`, `handler/handler.go`,
`internal/httputil/httputil.go` (new file)

**Problem:** Both `auth` and `handler` defined identical `writeError` helpers
(and `handler` also defined `writeJSON`). If the JSON error envelope ever
changed (e.g., adding a top-level `"code"` field alongside `"error"`), two
files would need to be updated in sync — with no compiler help to catch a
missed update.

**Change:** Created `internal/httputil/httputil.go` with two exported functions:

```go
func WriteJSON(w http.ResponseWriter, status int, data any)
func WriteError(w http.ResponseWriter, status int, msg string)
```

Updated `auth/auth.go` and `handler/handler.go` to call the shared helpers,
removing their local copies.

**Test:** No new test needed. The existing `auth` and `handler` test suites
exercise the full response format end-to-end; all tests passing after the
refactor is the acceptance criterion.

**Commit message:** `refactor: extract WriteJSON/WriteError into internal/httputil`

---

## Task 7 — Extract named constants for server timeouts in `main.go`

**Files:** `main.go`, `main_test.go`

**Problem:** `newServer` hard-coded `5 * time.Second`, `10 * time.Second`, and
`120 * time.Second` as inline literals. `TestNewServerTimeouts` duplicated
those same literals in its assertions. Tuning a timeout required updating two
files; missing one left the test silently asserting a stale value with no
indication anything was wrong.

**Change:** Declared three named constants at the package level:

```go
const (
    defaultReadTimeout  = 5 * time.Second
    defaultWriteTimeout = 10 * time.Second
    defaultIdleTimeout  = 120 * time.Second
)
```

Used them in `newServer`. Updated `TestNewServerTimeouts` to reference the
constants rather than literals, giving the test and the implementation a single
source of truth.

**Test:** `TestNewServerTimeouts` updated to reference the named constants. A
future timeout change requires editing only one location and the test stays
automatically correct.

**Commit message:** `refactor(main): extract named constants for server timeouts`

---

## Task 8 — Replace `defer` with `t.Cleanup` in `TestHealthUnhealthy`

**File:** `handler/handler_test.go`

**Problem:** `TestHealthUnhealthy` created a `httptest.Server` and tore it down
with `defer ts.Close()`. Every other test server in the same file used
`t.Cleanup(ts.Close)`, the pattern mandated by the project's testing
conventions. This inconsistency made the file harder to skim and could
theoretically cause cleanup ordering surprises in subtests.

**Change:** Replaced `defer ts.Close()` with `t.Cleanup(ts.Close)` in
`TestHealthUnhealthy`.

**Test:** The change is in the test file itself. All handler tests passing is
the acceptance criterion.

**Commit message:** `test(handler): replace defer with t.Cleanup in TestHealthUnhealthy`

---

## Task 9 — Clarify exported-fields exception for DTO types in `AGENTS.md`

**File:** `AGENTS.md`

**Problem:** The style guide stated *"Keep fields unexported. Use constructors
(`New`) to build instances rather than exporting struct fields."* However,
`store.URL` is a plain data-transfer struct whose fields (`ID`, `Code`,
`OriginalURL`, `CreatedAt`) are all exported and accessed directly by the
handler. Adding getter methods for every field would be pure ceremony with no
encapsulation benefit for this type. The code was correct as written, but it
appeared to contradict the stated convention — confusing for new contributors.

**Change:** Added a clarifying note under the **Structs** section:

> **Exception:** pure data-transfer types that carry no behaviour (e.g.
> `store.URL`) may use exported fields. The "unexported fields + constructor"
> rule applies to types that own significant logic or internal state.

Also updated the **JSON responses** section to reference `httputil.WriteJSON`
and `httputil.WriteError` now that the per-package helpers were consolidated in
`internal/httputil` as part of Task 6.

**Test:** Documentation-only change. `CGO_ENABLED=1 go test ./...` must remain
green.

**Commit message:** `docs(style): clarify exported-fields exception for DTO types`

---

## Execution order and dependency graph

```
Task 1  fix(store): CreatedAt timestamp   ─┐
Task 2  test(store): Ping tests           ─┤ all confined to store; independent
Task 3  fix(store): pool config order     ─┘ of each other
         ↓
Task 4  refactor(store+handler): ErrConflict sentinel
         ↓
Task 5  perf(shortener): charsetLen      ─┐
Task 6  refactor: internal/httputil      ─┤ independent of each other;
Task 7  refactor(main): timeouts         ─┤ run in any order
Task 8  test(handler): t.Cleanup         ─┤
Task 9  docs(style): AGENTS.md           ─┘
```

Finish Tasks 1–3 first (all confined to `store`), then Task 4 (spans `store`
and `handler`), then Tasks 5–9 in any order.

---

## Verification command

Run after every commit before pushing:

```bash
CGO_ENABLED=1 go test ./...
```

All packages must report `ok`.
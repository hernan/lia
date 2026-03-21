# Security & Stability Review

## Security Vulnerabilities

### S1. Predictable code generation

**File:** `shortener/shortener.go:15`
**Severity:** Medium

`math/rand` is not cryptographically secure. An attacker observing generated codes can potentially predict future ones. Should use `crypto/rand`.

**Fix:** Use `crypto/rand` with `math/big` for unbiased random index selection.

---

### S2. URL scheme not restricted

**File:** `handler/handler.go:47`
**Severity:** High

Validation allows any scheme. An attacker can store `javascript:alert(1)` or `data:text/html,...` URLs. The redirect at line 92 would execute them. Should restrict to `http` and `https` only.

**Fix:** Check `parsed.Scheme == "http" || parsed.Scheme == "https"` after parsing.

---

### S3. No request body size limit

**File:** `handler/handler.go:35`
**Severity:** High

`json.NewDecoder(r.Body).Decode(...)` reads unbounded input. An attacker can send a multi-GB payload to OOM the server. Should use `http.MaxBytesReader`.

**Fix:** Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, 1<<20)` (1MB limit).

---

### S4. No HTTP server timeouts

**File:** `main.go:77-80`
**Severity:** High

No `ReadTimeout`, `WriteTimeout`, or `IdleTimeout` set. Vulnerable to slowloris attacks — an attacker holds connections open indefinitely to exhaust resources.

**Fix:** Set `ReadTimeout: 5s`, `WriteTimeout: 10s`, `IdleTimeout: 120s` on `http.Server`.

---

### S5. Inconsistent error response format

**File:** `auth/auth.go:13`
**Severity:** Low

Auth middleware uses `http.Error()` (plain text), but handlers return JSON. Inconsistent content types can confuse clients and leak information about endpoint behavior.

**Fix:** Return JSON error responses from auth middleware with `Content-Type: application/json`.

---

## Stability Issues

### T1. Shutdown error ignored

**File:** `main.go:89`
**Severity:** Low

`srv.Shutdown(ctx)` return value is discarded. If shutdown fails, it's silently swallowed.

**Fix:** Capture and log the error.

---

### T2. Fragile constraint error detection

**File:** `handler/handler.go:113-114`
**Severity:** Low

`strings.Contains(err.Error(), "UNIQUE constraint failed")` depends on SQLite's English error message text. Could break with locale changes or driver updates. The `sqlite3` driver exposes typed errors that would be more robust.

**Fix:** Import `github.com/mattn/go-sqlite3` in handler, use `sqlite3.ErrCode(err) == sqlite3.ErrConstraint`.

---

### T3. No connection pool configuration

**File:** `store/store.go:21`
**Severity:** Low

`sql.Open` uses default pool settings. Under load, unbounded connections could exhaust file descriptors. Should set `SetMaxOpenConns`, `SetMaxIdleConns`, and `SetConnMaxLifetime`.

**Fix:** Set `db.SetMaxOpenConns(1)`, `db.SetMaxIdleConns(1)`, `db.SetConnMaxLifetime(5 * time.Minute)`. SQLite is single-writer, so 1 open conn prevents write contention.

---

### T4. Code space exhaustion risk (skipped)

**File:** `shortener/shortener.go:8-9`
**Severity:** Low

62^6 ≈ 56.8B combinations. At high insert rates with random generation, collision probability grows. The 5-retry limit could eventually be hit under sustained load. Deferred for now.

---

## Implementation Plan

Execution order: Security fixes first (S3 → S4 → S2 → S1 → S5), then stability (T1 → T2 → T3).

| # | Unit | File | Test | Commit Prefix |
|---|------|------|------|---------------|
| S3 | Request body size limit (1MB) | `handler/handler.go:35` | POST oversized body → 413 | `Security:` |
| S4 | HTTP server timeouts | `main.go:77-80` | Config verification | `Security:` |
| S2 | Restrict URL scheme to http/https | `handler/handler.go:47` | `javascript:`, `file:`, `data:` → 400 | `Security:` |
| S1 | Use crypto/rand | `shortener/shortener.go` | Existing tests pass | `Security:` |
| S5 | JSON errors in auth middleware | `auth/auth.go` | Content-Type is `application/json` | `Security:` |
| T1 | Log shutdown error | `main.go:89` | No test (trivial) | `Stability:` |
| T2 | Typed SQLite error codes | `handler/handler.go:113` | Existing collision test passes | `Stability:` |
| T3 | Connection pool config | `store/store.go` | Verify pool settings | `Stability:` |

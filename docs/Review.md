# Code Review Implementation Plan

## Critical Issues

### Unit 1: Fix timing-attack in auth

**File:** `auth/auth.go`
**Change:** Replace `parts[1] != token` with `crypto/subtle.ConstantTimeCompare`
**Test:** Existing `auth_test.go` already covers valid/invalid tokens. Add a test verifying the fix doesn't break case sensitivity or empty token edge cases if needed.
**Commit:** `Review: use constant-time comparison for token validation`

---

### Unit 2: Handle LastInsertId error in store

**File:** `store/store.go:52`
**Change:** Check the error from `result.LastInsertId()` and return it
**Test:** Add a test in `store_test.go` verifying `Create` returns the populated ID (existing tests already do this implicitly — make it explicit). The error path is hard to trigger with SQLite, so document it.
**Commit:** `Review: handle LastInsertId error in store.Create`

---

### Unit 3: Fix time inconsistency in store.Create

**File:** `store/store.go:57`
**Change:** Use `time.Now().UTC()` to match SQLite's `CURRENT_TIMESTAMP` behavior
**Test:** Add a test in `store_test.go` that creates a URL and verifies the returned `CreatedAt` is within a small delta of the DB-stored value (queried back).
**Commit:** `Review: use UTC time in store.Create to match SQLite`

---

## Maintainability Issues

### Unit 4: Add URL validation in handler

**File:** `handler/handler.go:37-41`
**Change:** After trimming, validate with `net/url.Parse()` and check for a non-empty scheme (e.g., `http` or `https`)
**Test:** Add cases in `handler_test.go` for `"not a url"`, `""`, `"ftp://valid"`, `"https://valid.com"` — verifying correct status codes.
**Commit:** `Review: validate URL format in Shorten handler`

---

### Unit 5: Handle code generation collisions

**File:** `handler/handler.go:43-47`
**Change:** Detect UNIQUE constraint violation errors from SQLite and retry with a new code (with a max retry limit). Extract code generation into a loop.
**Test:** Add a test in `handler_test.go` using a mock store that fails once with a UNIQUE constraint error then succeeds, verifying the handler retries and returns 201.
**Commit:** `Review: retry on code collision in Shorten handler`

---

### Unit 6: Extract writeError helper

**File:** `handler/handler.go`
**Change:** Add `writeError(w, status, msg)` that wraps the repeated `map[string]string{"error": msg}` pattern. Refactor existing error responses to use it.
**Test:** Existing handler tests already verify error status codes and JSON structure. Refactor is validated by existing tests passing.
**Commit:** `Review: extract writeError helper in handler`

---

### Unit 7: Make port and shutdown timeout configurable

**File:** `main.go`
**Change:** Read `PORT` (default `8080`) and `SHUTDOWN_TIMEOUT` (default `5s`) from env vars
**Test:** Unit-testing `main()` directly is awkward. Add a test that verifies env var parsing logic by extracting config loading into a testable `loadConfig()` function, then test that function.
**Commit:** `Review: make port and shutdown timeout configurable`

---

## Test Issues

### Unit 8: Fix flaky uniqueness test

**File:** `shortener/shortener_test.go:27-42`
**Change:** Remove the probabilistic `TestGenerateUniqueness` or replace it with a deterministic test that verifies the generator uses the full charset and correct length (which indirectly covers quality).
**Test:** The replacement test should be deterministic — e.g., run N generations and verify every character is from `Charset` and length is correct (existing `TestGenerateCharset` already does this, so the uniqueness test can simply be removed).
**Commit:** `Review: remove flaky probabilistic uniqueness test`

---

## Execution Order

Units 1-3 are independent and can be done in parallel. Unit 4 should come before Unit 5 (both touch the Shorten handler). Unit 6 is pure refactor after 4+5. Unit 7 is independent. Unit 8 is independent.

```
1 ─┐
2  ├─→ 4 → 5 → 6
3 ─┘            ↘
                 7
8 (anytime)
```

---

**Total: 8 units, 8 commits.** Each is independently testable with `CGO_ENABLED=1 go test ./...`.

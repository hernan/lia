# Admin Panel — Code Review & Fix Plan

## Overview

This document records every issue found during the code review of the `admin`
package (`admin/admin.go`, `admin/admin_test.go`, and
`internal/session/session.go`).  Issues are grouped by severity.  Each task is
**atomic**: one root cause → one fix → one test → one commit prefixed with
`FIX:`.

---

## Issues Found

### 🔴 Security

#### S-1 · CSRF token compared with plain `==` (timing-attack vulnerability)
**File:** `internal/session/session.go` → `VerifyCSRF`

Plain string comparison (`==`) leaks timing information.  An attacker who can
measure response latency can brute-force the token one character at a time.
The rest of the codebase already uses `subtle.ConstantTimeCompare` for
credential checks (`admin.go`); CSRF tokens require the same treatment.

#### S-2 · `POST /admin/logout` has no CSRF middleware
**File:** `admin/admin.go` → `RegisterRoutes`

The logout route is wrapped with `requireAuth` but **not** `requireCSRF`.  Any
third-party page can force an authenticated admin to log out via a crafted
request.  The `dashboard.html` template already embeds a CSRF token in the
logout form — it just is not verified on the server.

#### S-3 · Session secret derived from the API bearer token
**File:** `main.go`

The session-signing secret is computed as `sha256(SHORTENER_TOKEN)`.  If the
API token leaks (e.g. in a log line or client request), an attacker can compute
the HMAC secret and forge valid session cookies.  The two secrets serve
different purposes and must be independent.

---

### 🟠 Correctness

#### C-4 · `CreateURL` detects conflicts via `strings.Contains` on error message
**File:** `admin/admin.go` → `CreateURL`

The project already defines `store.ErrConflict` and `store.Create` wraps it
with `fmt.Errorf("%w", ErrConflict)`.  The current check
`strings.Contains(err.Error(), "code already exists")` is fragile: it breaks if
the message ever changes and does not unwrap errors from future backends.  The
correct idiom is `errors.Is(err, store.ErrConflict)`.

#### C-5 · All `ExecuteTemplate` calls ignore the returned error
**File:** `admin/admin.go` — 5 call sites

If a template fails mid-render the HTTP response is already committed with
`200 OK` and a truncated body, with no log entry.  The robust fix is to render
into a `bytes.Buffer` first; on success flush the buffer to `w`; on failure log
and return `500`.

---

### 🟡 Code Quality / Convention (AGENTS.md)

#### Q-6 · Tests use `defer srv.Close()` instead of `t.Cleanup` (×15)
**File:** `admin/admin_test.go`

AGENTS.md explicitly requires: *"Use `t.Cleanup()` for resource cleanup instead
of `defer`."*  Every one of the 15 test functions repeats this violation.

#### Q-7 · Dead `_ = created` and `_ = sm` assignments
**Files:** `admin/admin.go`, `admin/admin_test.go`

`_ = created` in `CreateURL` discards the freshly created record without using
it.  `_ = sm` in `TestLoginPostInvalidCredentials` is a copy-paste artifact —
`sm` is never needed in that test.  Both are noise that obscures intent.

---

## Fix Plan

Each task below is self-contained and must leave the test suite green before
the commit is made.

---

### Task 1 — Constant-time CSRF comparison

| | |
|---|---|
| **Scope** | `internal/session/session.go` |
| **Fix** | Replace `cookieToken == formToken` with `subtle.ConstantTimeCompare`. |
| **New test** | `TestVerifyCSRFConstantTimeEqual` — equal tokens return `true`; `TestVerifyCSRFConstantTimeNotEqual` — unequal tokens return `false`. |
| **Commit** | `FIX: use constant-time comparison in VerifyCSRF to prevent timing attacks` |

---

### Task 2 — CSRF protection on logout

| | |
|---|---|
| **Scope** | `admin/admin.go` → `RegisterRoutes` |
| **Fix** | Wrap the logout route with `a.requireCSRF`. |
| **New test** | `TestLogoutRequiresCSRF` — POST to `/admin/logout` without CSRF token must return `403 Forbidden`. |
| **Commit** | `FIX: protect logout endpoint with CSRF middleware` |

---

### Task 3 — Decouple session secret from API token

| | |
|---|---|
| **Scope** | `main.go`, `main_test.go` |
| **Fix** | Add `ADMIN_SESSION_SECRET` env var to `config`. Use it as the session signing key. Fall back to `sha256(token)` with a warning log when the variable is absent. |
| **New tests** | `TestLoadConfigAdminSessionSecret` — field is populated from env; `TestLoadConfigAdminSessionSecretFallback` — fallback value when env is unset. |
| **Commit** | `FIX: add ADMIN_SESSION_SECRET env var to decouple session key from API token` |

---

### Task 4 — Use `errors.Is` for conflict detection in `CreateURL`

| | |
|---|---|
| **Scope** | `admin/admin.go` → `CreateURL` |
| **Fix** | Replace `strings.Contains(err.Error(), "code already exists")` with `errors.Is(err, store.ErrConflict)`. Remove unused `strings` import if it becomes orphaned. |
| **New tests** | `TestCreateURLConflictRetries` — mock returns `ErrConflict` on first call then succeeds; handler must redirect `303`. `TestCreateURLConflictExhausted` — mock always returns `ErrConflict`; handler must return `500`. |
| **Commit** | `FIX: use errors.Is(store.ErrConflict) instead of string matching in CreateURL` |

---

### Task 5 — Handle `ExecuteTemplate` errors with a buffered render helper

| | |
|---|---|
| **Scope** | `admin/admin.go` |
| **Fix** | Introduce `func (a *Admin) render(w http.ResponseWriter, name string, data any) error` that writes to a `bytes.Buffer`. On success, copy the buffer to `w`. On failure, log and write `500`. Replace all bare `ExecuteTemplate` calls with `render`. |
| **New test** | `TestRenderTemplateError` — replace `a.tmpls` with a template set that triggers an execution error; all handler endpoints that render HTML must return `500` instead of `200` with a broken body. |
| **Commit** | `FIX: handle template execution errors with buffered render helper` |

---

### Task 6 — Replace `defer` with `t.Cleanup` in admin tests

| | |
|---|---|
| **Scope** | `admin/admin_test.go` |
| **Fix** | Replace all 15 occurrences of `defer srv.Close()` with `t.Cleanup(srv.Close)`. |
| **Test** | The existing suite is the test — it must remain green. |
| **Commit** | `FIX: replace defer srv.Close() with t.Cleanup per AGENTS.md conventions` |

---

### Task 7 — Remove dead `_ = created` and `_ = sm` assignments

| | |
|---|---|
| **Scope** | `admin/admin.go`, `admin/admin_test.go` |
| **Fix** | In `CreateURL` change `created, err =` to `_, err =` (the value is unused). In `TestLoginPostInvalidCredentials` change `a, sm :=` to `a, _ :=`. |
| **Test** | `go vet ./...` must report no unused-variable warnings after the change. |
| **Commit** | `FIX: remove dead _ = created and _ = sm assignments` |

---

## Priority Summary

| # | Severity | Issue | File(s) |
|---|----------|-------|---------|
| S-1 | 🔴 Security | CSRF timing attack via `==` comparison | `session/session.go` |
| S-2 | 🔴 Security | Logout endpoint missing CSRF protection | `admin/admin.go` |
| S-3 | 🔴 Security | Session secret coupled to API token | `main.go` |
| C-4 | 🟠 Correctness | `errors.Is` vs. string match for `ErrConflict` | `admin/admin.go` |
| C-5 | 🟠 Correctness | Template errors silently swallowed | `admin/admin.go` |
| Q-6 | 🟡 Convention | `defer` instead of `t.Cleanup` (×15) | `admin/admin_test.go` |
| Q-7 | 🟡 Convention | Dead `_ =` assignments | both |
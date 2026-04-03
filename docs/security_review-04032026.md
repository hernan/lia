# Security Review — 04/03/2026

## Overview

Review of the urlshortener Go URL shortener API covering authentication, session management, input validation, and common web vulnerabilities.

---

## ✅ What's Done Well

| Area | Detail |
|------|--------|
| **Auth timing-safe** | `crypto/subtle.ConstantTimeCompare` used for bearer token, admin login, and CSRF verification |
| **CSRF protection** | 32-byte random tokens via `crypto/rand`, required on all state-changing admin POSTs |
| **Session cookies** | `HttpOnly: true`, `SameSite: Lax`, HMAC-SHA256 signed |
| **Input validation** | URL scheme/host validation, `http.MaxBytesReader` (1MB limit) on request body |
| **SQL injection** | Parameterized queries with `?` placeholders throughout |
| **XSS prevention** | `html/template` auto-escapes all template output |
| **Code generation** | Cryptographically secure random via `crypto/rand` |
| **Server timeouts** | Read (5s), Write (10s), Idle (120s) configured — prevents slowloris |
| **Error responses** | Generic messages ("internal error") — no stack traces or internals leaked |

---

## ⚠️ Issues Found

### High Priority

#### 1. Session cookie not HTTPS-only

**File:** `internal/session/session.go:41`

```go
Secure: false,
```

Cookie will be sent over plaintext HTTP. Should be `true` in production.

#### 2. No rate limiting on `/admin/login`

**File:** `admin/admin.go:127`

Brute-force attack against admin credentials is unmitigated.

#### 3. No rate limiting on `/shorten`

**File:** `handler/handler.go:35`

Authenticated but no request throttling — could be abused to fill the database.

### Medium Priority

#### 4. Missing security headers

No `X-Content-Type-Options`, `X-Frame-Options`, or `Content-Security-Policy` headers set anywhere.

#### 5. Admin password stored in plaintext

**File:** `main.go:76`

`ADMIN_PASSWORD` compared directly (though constant-time). No hashing/salting.

#### 6. Flash messages injectable via URL

**File:** `admin/admin.go:183`

```go
flash := r.URL.Query().Get("flash")
```

Anyone can craft URLs with arbitrary flash messages shown to authenticated admins.

#### 7. Session secret fallback couples security domains

**File:** `main.go:64-66`

When `ADMIN_SESSION_SECRET` is unset, it derives from `SHORTENER_TOKEN`. Compromise of one affects the other.

### Low Priority

#### 8. No audit logging

Admin actions (create/update/delete) are not logged.

#### 9. Open redirect surface

`/{code}` redirects to any stored URL. While validated on creation, a compromised admin account could store malicious redirect targets.

#### 10. No `Secure` flag on CSRF cookie

**File:** `internal/session/session.go:92`

Same issue as session cookie — sent over HTTP.

---

## Summary

The **core crypto and auth patterns are solid** — constant-time comparisons, HMAC-signed sessions, CSRF tokens, parameterized queries, and `html/template` usage are all correct.

The **biggest gaps** are operational: no rate limiting, missing security headers, and the `Secure: false` cookie flag. These are relatively straightforward to fix but important before any production deployment.

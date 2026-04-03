# Plan: Replace URL Flash Messages with Session-Based Flash

## Problem

Flash messages are currently passed via URL query parameters (`?flash=URL+created&flash_error=1`) on redirects. This has two issues:

1. **Security**: Flash messages are injectable — anyone can craft a link like `/admin?flash=arbitrary+message` and an authenticated admin will see it.
2. **UX**: URLs are cluttered with flash parameters.

## Solution

Move flash message storage into the existing HMAC-signed session cookie, switching the cookie payload from pipe-delimited format to JSON for future extensibility.

## Cookie Format Change

### Before
```
username|hmac(username)
```

### After
```json
{"u":"admin","f":"","fe":false}|hmac(json_blob)
```

| Key | Type | Description |
|-----|------|-------------|
| `u` | string | Username |
| `f` | string | Flash message (empty when none) |
| `fe` | bool | Flash is an error |

Short keys minimize cookie size while keeping the payload self-describing and easy to extend.

## Changes

### 1. `internal/session/session.go`

**Add struct:**
```go
type sessionData struct {
    Username   string `json:"u"`
    Flash      string `json:"f"`
    FlashError bool   `json:"fe"`
}
```

**Modify existing methods:**

| Method | Change |
|--------|--------|
| `Create(username)` | Marshal `sessionData{Username: username}` to JSON, sign, set as cookie value |
| `Validate(r)` | Parse JSON from cookie, verify HMAC, return `(username, flash, flashError, error)` |

**Add new methods:**

| Method | Purpose |
|--------|---------|
| `SetFlash(w, msg string, isError bool)` | Read current cookie, update flash fields, re-sign, write new cookie |
| `ClearFlash(w)` | Read current cookie, clear flash fields, re-sign, write new cookie |

### 2. `admin/admin.go`

**`Dashboard` handler:**
- Remove: `flash := r.URL.Query().Get("flash")` and `flashError := r.URL.Query().Get("flash_error") == "1"`
- Read flash from session via updated `Validate()` return values
- After rendering: call `ClearFlash(w)` so flash doesn't persist on refresh

**`CreateURL`, `UpdateURL`, `DeleteURL` handlers:**
- Replace `http.Redirect(..., "/admin?flash=...")` with:
  ```go
  a.sessions.SetFlash(w, "message", isError)
  http.Redirect(w, r, "/admin", http.StatusSeeOther)
  ```

### 3. Breaking Change

Switching from pipe-delimited to JSON format invalidates all existing session cookies. Users will be redirected to login on their next request. This is acceptable because:
- Sessions are 24h anyway
- No data loss — sessions only store auth identity
- Communicated as a security improvement

## Test Updates

### `internal/session/session_test.go`
- Update cookie value assertions (pipe → JSON)
- Update malformed cookie test (no longer needs pipe check)
- Add tests for `SetFlash` and `ClearFlash`
- Add test: flash survives round-trip through cookie
- Add test: ClearFlash zeroes flash fields but preserves username

### `admin/admin_test.go`
- `TestCreateURLInvalidURL`: Remove assertion on `flash_error=1` in redirect URL
- Add test: flash message appears after create/update/delete
- Add test: flash message is cleared after dashboard render

## File Change Summary

| File | Lines Changed | Nature |
|------|---------------|--------|
| `internal/session/session.go` | ~50 added, ~15 modified | JSON format + flash methods |
| `internal/session/session_test.go` | ~30 added, ~10 modified | New format + flash tests |
| `admin/admin.go` | ~10 added, ~10 modified | Flash via session instead of URL |
| `admin/admin_test.go` | ~20 modified | Updated assertions |
| **Total** | **~80 added, ~45 modified** | |

## Effort Estimate

| Area | Time |
|------|------|
| `session.go` — JSON refactor + flash methods | 45 min |
| `admin.go` — update 5 handlers | 20 min |
| Session tests | 20 min |
| Admin tests | 20 min |
| Run tests + fix issues | 15 min |
| **Total** | **~2 hours** |

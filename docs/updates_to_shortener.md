# Shortener Refactor: Base62 → Base32

## Motivation

The current code generator uses a base62 charset with per-character `crypto/rand.Int()` calls. Each call involves `big.Int` modular reduction, resulting in 6 separate `big.Int` operations per code. Base32 allows a single `crypto/rand.Read()` call with bit-aligned modular indexing — the same pattern used internally by `crypto/rand.Text()` (added in Go 1.24).

## Changes

### Unit 1: Replace base62 generator with base32 modular indexing

**File:** `shortener/shortener.go`

**Before:**
```go
const (
    Charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // 62 chars
    Length  = 6
)

func Generate() string {
    b := make([]byte, Length)
    charsetLen := big.NewInt(int64(len(Charset)))
    for i := range b {
        n, err := rand.Int(rand.Reader, charsetLen)
        if err != nil {
            panic("crypto/rand failed: " + err.Error())
        }
        b[i] = Charset[n.Int64()]
    }
    return string(b)
}
```

**After:**
```go
const (
    Charset = "abcdefghjkmnpqrstuvwxyz023456789" // 32 chars, no l/i/o/1
    Length  = 6
)

func Generate() string {
    b := make([]byte, Length)
    if _, err := rand.Read(b); err != nil {
        panic("crypto/rand failed: " + err.Error())
    }
    for i := range b {
        b[i] = Charset[b[i]%32]
    }
    return string(b)
}
```

**Changes:**
- Charset reduced from 62 to 32 characters (lowercase, unambiguous: no `l`, `i`, `o`, `1`)
- Removed `math/big` dependency
- Single `rand.Read()` call instead of 6 `rand.Int()` calls
- Modular indexing (`b[i]%32`) instead of `big.Int` arithmetic
- ~6x fewer cryptographic operations per code

**Test updates:**
- `TestGenerateCharset` — uses exported `Charset` constant, automatically validates against new alphabet
- `TestGenerateLength` — unchanged, still checks length is 6
- `contains` helper — still works with new charset

**Impact on other packages:**
- `handler/handler_test.go` — uses in-memory store, no charset assumptions
- `store/store_test.go` — uses hardcoded codes, no generator dependency

**Commit:** `Refactor: use base32 modular indexing for code generation`

## Tradeoffs

| Aspect | Base62 (before) | Base32 (after) |
|--------|-----------------|----------------|
| Charset size | 62 | 32 |
| Code space | 62^6 ≈ 56.8B | 32^6 ≈ 1.07B |
| rand calls per code | 6 (`rand.Int`) | 1 (`rand.Read`) |
| Big.Int operations | 6 | 0 |
| Case sensitive | Yes | No |
| Ambiguous chars | Yes (l/I/1/O/0) | No |

## References

- `crypto/rand.Text()` source — uses same modular indexing pattern
- [RFC 4648 Base32](https://tools.ietf.org/html/rfc4648) — standard base32 alphabet
- Go 1.24 release notes — `crypto/rand.Text()` addition

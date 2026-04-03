package session

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var testSecret = []byte("test-secret-key-for-hmac")

func TestCreateAndValidate(t *testing.T) {
	m := New(testSecret)
	cookie := m.Create("admin")

	if cookie.Name != "session" {
		t.Errorf("expected cookie name session, got %s", cookie.Name)
	}

	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		t.Fatalf("expected cookie value to contain |, got %s", cookie.Value)
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode cookie data: %v", err)
	}

	var data struct {
		Username string `json:"u"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("failed to unmarshal cookie data: %v", err)
	}
	if data.Username != "admin" {
		t.Errorf("expected admin, got %s", data.Username)
	}

	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(cookie)

	username, _, _, err := m.Validate(r)
	if err != nil {
		t.Fatal(err)
	}
	if username != "admin" {
		t.Errorf("expected admin, got %s", username)
	}
}

func TestValidateNoCookie(t *testing.T) {
	m := New(testSecret)
	r := httptest.NewRequest("GET", "/admin", nil)

	_, _, _, err := m.Validate(r)
	if err == nil {
		t.Fatal("expected error when no cookie present")
	}
}

func TestValidateTamperedValue(t *testing.T) {
	m := New(testSecret)
	cookie := m.Create("admin")
	cookie.Value = `{"u":"hacker"}|badmac`

	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(cookie)

	_, _, _, err := m.Validate(r)
	if err == nil {
		t.Fatal("expected error for tampered cookie")
	}
}

func TestValidateDifferentSecret(t *testing.T) {
	m1 := New([]byte("secret-one"))
	cookie := m1.Create("admin")

	m2 := New([]byte("secret-two"))
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(cookie)

	_, _, _, err := m2.Validate(r)
	if err == nil {
		t.Fatal("expected error with different secret")
	}
}

func TestValidateMalformedCookie(t *testing.T) {
	m := New(testSecret)
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "no-pipe-separator"})

	_, _, _, err := m.Validate(r)
	if err == nil {
		t.Fatal("expected error for malformed cookie")
	}
}

func TestValidateMalformedJSON(t *testing.T) {
	m := New(testSecret)
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "not-json|badmac"})

	_, _, _, err := m.Validate(r)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestDestroy(t *testing.T) {
	m := New(testSecret)
	w := httptest.NewRecorder()
	m.Destroy(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "session" {
		t.Errorf("expected session, got %s", c.Name)
	}
	if c.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", c.MaxAge)
	}
}

func TestSetFlashAndClearFlash(t *testing.T) {
	m := New(testSecret)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(m.Create("admin"))

	m.SetFlash(w, r, "something happened", true)

	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	newReq := httptest.NewRequest("GET", "/admin", nil)
	newReq.AddCookie(cookies[0])

	_, flash, flashError, err := m.Validate(newReq)
	if err != nil {
		t.Fatal(err)
	}
	if flash != "something happened" {
		t.Errorf("expected flash message, got %q", flash)
	}
	if !flashError {
		t.Error("expected flash error to be true")
	}

	clearW := httptest.NewRecorder()
	m.ClearFlash(clearW, newReq)

	clearReq := httptest.NewRequest("GET", "/admin", nil)
	clearReq.AddCookie(clearW.Result().Cookies()[0])

	_, flash, flashError, err = m.Validate(clearReq)
	if err != nil {
		t.Fatal(err)
	}
	if flash != "" {
		t.Errorf("expected empty flash after clear, got %q", flash)
	}
	if flashError {
		t.Error("expected flash error to be false after clear")
	}
}

func TestFlashRoundTrip(t *testing.T) {
	m := New(testSecret)
	createW := httptest.NewRecorder()
	http.SetCookie(createW, m.Create("admin"))

	createReq := httptest.NewRequest("GET", "/admin", nil)
	createReq.AddCookie(createW.Result().Cookies()[0])

	flashW := httptest.NewRecorder()
	m.SetFlash(flashW, createReq, "URL created", false)

	flashReq := httptest.NewRequest("GET", "/admin", nil)
	flashReq.AddCookie(flashW.Result().Cookies()[0])

	_, flash, flashError, err := m.Validate(flashReq)
	if err != nil {
		t.Fatal(err)
	}
	if flash != "URL created" {
		t.Errorf("expected 'URL created', got %q", flash)
	}
	if flashError {
		t.Error("expected flash error to be false")
	}
}

func TestClearFlashPreservesUsername(t *testing.T) {
	m := New(testSecret)
	createW := httptest.NewRecorder()
	http.SetCookie(createW, m.Create("testuser"))

	createReq := httptest.NewRequest("GET", "/admin", nil)
	createReq.AddCookie(createW.Result().Cookies()[0])

	flashW := httptest.NewRecorder()
	m.SetFlash(flashW, createReq, "test message", true)

	flashReq := httptest.NewRequest("GET", "/admin", nil)
	flashReq.AddCookie(flashW.Result().Cookies()[0])

	clearW := httptest.NewRecorder()
	m.ClearFlash(clearW, flashReq)

	clearReq := httptest.NewRequest("GET", "/admin", nil)
	clearReq.AddCookie(clearW.Result().Cookies()[0])

	username, flash, flashError, err := m.Validate(clearReq)
	if err != nil {
		t.Fatal(err)
	}
	if username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", username)
	}
	if flash != "" {
		t.Errorf("expected empty flash, got %q", flash)
	}
	if flashError {
		t.Error("expected flash error to be false")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(token))
	}

	token2, _ := GenerateToken()
	if token == token2 {
		t.Error("expected unique tokens")
	}
}

func TestSetCSRFCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetCSRFCookie(w, "mytoken")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "csrf_token" {
		t.Errorf("expected csrf_token, got %s", c.Name)
	}
	if c.Value != "mytoken" {
		t.Errorf("expected mytoken, got %s", c.Value)
	}
	if c.HttpOnly {
		t.Error("expected non-httponly CSRF cookie")
	}
}

func TestVerifyCSRFSucceeds(t *testing.T) {
	form := url.Values{}
	form.Set("csrf_token", "valid-token")

	r := httptest.NewRequest("POST", "/admin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "valid-token"})

	if !VerifyCSRF(r) {
		t.Error("expected CSRF verification to succeed")
	}
}

func TestVerifyCSRFFailsMissingCookie(t *testing.T) {
	form := url.Values{}
	form.Set("csrf_token", "valid-token")

	r := httptest.NewRequest("POST", "/admin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if VerifyCSRF(r) {
		t.Error("expected CSRF verification to fail without cookie")
	}
}

func TestVerifyCSRFFailsMismatch(t *testing.T) {
	form := url.Values{}
	form.Set("csrf_token", "wrong-token")

	r := httptest.NewRequest("POST", "/admin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "correct-token"})

	if VerifyCSRF(r) {
		t.Error("expected CSRF verification to fail with mismatched tokens")
	}
}

func TestVerifyCSRFFailsMissingForm(t *testing.T) {
	r := httptest.NewRequest("POST", "/admin", nil)
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "some-token"})

	if VerifyCSRF(r) {
		t.Error("expected CSRF verification to fail without form token")
	}
}

func TestVerifyCSRFConstantTimeEqual(t *testing.T) {
	token := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	form := url.Values{}
	form.Set("csrf_token", token)

	r := httptest.NewRequest("POST", "/admin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})

	if !VerifyCSRF(r) {
		t.Error("expected VerifyCSRF to return true for matching tokens")
	}
}

func TestVerifyCSRFConstantTimeNotEqual(t *testing.T) {
	form := url.Values{}
	form.Set("csrf_token", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	r := httptest.NewRequest("POST", "/admin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "csrf_token", Value: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})

	if VerifyCSRF(r) {
		t.Error("expected VerifyCSRF to return false for mismatched tokens")
	}
}

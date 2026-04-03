package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	cookieName = "session"
	csrfName   = "csrf_token"
	cookiePath = "/admin"
	cookieExp  = 24 * time.Hour
)

type sessionData struct {
	Username   string `json:"u"`
	Flash      string `json:"f"`
	FlashError bool   `json:"fe"`
}

// Manager handles session creation, validation, and CSRF tokens.
type Manager struct {
	secret []byte
}

// New creates a Manager with the given secret for HMAC signing.
func New(secret []byte) *Manager {
	return &Manager{secret: secret}
}

// Create generates a session cookie for the given username.
func (m *Manager) Create(username string) *http.Cookie {
	data := sessionData{Username: username}
	payload := m.marshal(data)
	return &http.Cookie{
		Name:     cookieName,
		Value:    payload,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cookieExp),
	}
}

// Validate checks the session cookie and returns the username, flash message,
// and flash error flag. Returns an error if the cookie is missing or invalid.
func (m *Manager) Validate(r *http.Request) (username string, flash string, flashError bool, err error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", "", false, fmt.Errorf("no session cookie")
	}

	parts := strings.SplitN(c.Value, "|", 2)
	if len(parts) != 2 {
		return "", "", false, fmt.Errorf("malformed session cookie")
	}

	encoded, mac := parts[0], parts[1]
	expected := m.sign(encoded)
	if !hmac.Equal([]byte(mac), []byte(expected)) {
		return "", "", false, fmt.Errorf("invalid session signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false, fmt.Errorf("malformed session data")
	}

	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", false, fmt.Errorf("malformed session data")
	}

	return data.Username, data.Flash, data.FlashError, nil
}

// Destroy clears the session cookie.
func (m *Manager) Destroy(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     cookiePath,
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// SetFlash stores a flash message in the session cookie.
func (m *Manager) SetFlash(w http.ResponseWriter, r *http.Request, msg string, isError bool) {
	data := m.readCookie(r)
	data.Flash = msg
	data.FlashError = isError
	m.writeCookie(w, data)
}

// ClearFlash removes any flash message from the session cookie.
func (m *Manager) ClearFlash(w http.ResponseWriter, r *http.Request) {
	data := m.readCookie(r)
	data.Flash = ""
	data.FlashError = false
	m.writeCookie(w, data)
}

func (m *Manager) readCookie(r *http.Request) sessionData {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return sessionData{}
	}
	parts := strings.SplitN(c.Value, "|", 2)
	if len(parts) != 2 {
		return sessionData{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return sessionData{}
	}
	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return sessionData{}
	}
	return data
}

func (m *Manager) writeCookie(w http.ResponseWriter, data sessionData) {
	payload := m.marshal(data)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    payload,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cookieExp),
	})
}

func (m *Manager) marshal(data sessionData) string {
	b, err := json.Marshal(data)
	if err != nil {
		panic("session: failed to marshal session data")
	}
	encoded := base64.RawURLEncoding.EncodeToString(b)
	mac := m.sign(encoded)
	return encoded + "|" + mac
}

// GenerateToken creates a random CSRF token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetCSRFCookie sets the CSRF token as a non-httponly cookie so the
// token can be read and included in forms.
func SetCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfName,
		Value:    token,
		Path:     cookiePath,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cookieExp),
	})
}

// TokenFromRequest extracts the CSRF token from the cookie.
func TokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(csrfName)
	if err != nil {
		return ""
	}
	return c.Value
}

// VerifyCSRF compares the form value "csrf_token" against the cookie using
// constant-time comparison to prevent timing attacks.
func VerifyCSRF(r *http.Request) bool {
	cookieToken := TokenFromRequest(r)
	if cookieToken == "" {
		return false
	}
	formToken := r.FormValue("csrf_token")
	if formToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(formToken)) == 1
}

func (m *Manager) sign(username string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(username))
	return hex.EncodeToString(mac.Sum(nil))
}

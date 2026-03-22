package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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
	mac := m.sign(username)
	value := username + "|" + mac
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cookieExp),
	}
}

// Validate checks the session cookie and returns the username.
// Returns an error if the cookie is missing or invalid.
func (m *Manager) Validate(r *http.Request) (string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", fmt.Errorf("no session cookie")
	}

	parts := strings.SplitN(c.Value, "|", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed session cookie")
	}

	username, mac := parts[0], parts[1]
	expected := m.sign(username)
	if !hmac.Equal([]byte(mac), []byte(expected)) {
		return "", fmt.Errorf("invalid session signature")
	}

	return username, nil
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

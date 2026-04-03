package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"urlshortener/admin"
	"urlshortener/auth"
	"urlshortener/handler"
	"urlshortener/internal/session"
	"urlshortener/store"
)

func TestNewServerTimeouts(t *testing.T) {
	cfg := config{addr: ":9999"}
	srv := newServer(cfg, http.NewServeMux())

	if srv.ReadTimeout != defaultReadTimeout {
		t.Errorf("expected ReadTimeout %v, got %v", defaultReadTimeout, srv.ReadTimeout)
	}
	if srv.WriteTimeout != defaultWriteTimeout {
		t.Errorf("expected WriteTimeout %v, got %v", defaultWriteTimeout, srv.WriteTimeout)
	}
	if srv.IdleTimeout != defaultIdleTimeout {
		t.Errorf("expected IdleTimeout %v, got %v", defaultIdleTimeout, srv.IdleTimeout)
	}
	if srv.Addr != ":9999" {
		t.Errorf("expected Addr :9999, got %s", srv.Addr)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "mytoken")
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("PORT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.token != "mytoken" {
		t.Errorf("expected token mytoken, got %s", cfg.token)
	}
	if cfg.dbDriver != "sqlite" {
		t.Errorf("expected dbDriver sqlite, got %s", cfg.dbDriver)
	}
	if cfg.dbDsn != "shortener.db" {
		t.Errorf("expected dbDsn shortener.db, got %s", cfg.dbDsn)
	}
	if cfg.addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", cfg.addr)
	}
	if cfg.shutdownTimeout != 5*time.Second {
		t.Errorf("expected shutdownTimeout 5s, got %v", cfg.shutdownTimeout)
	}
}

func TestLoadConfigCustom(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "secret")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DSN", "/tmp/test.db")
	t.Setenv("PORT", "9090")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.token != "secret" {
		t.Errorf("expected token secret, got %s", cfg.token)
	}
	if cfg.dbDriver != "sqlite" {
		t.Errorf("expected dbDriver sqlite, got %s", cfg.dbDriver)
	}
	if cfg.dbDsn != "/tmp/test.db" {
		t.Errorf("expected dbDsn /tmp/test.db, got %s", cfg.dbDsn)
	}
	if cfg.addr != ":9090" {
		t.Errorf("expected addr :9090, got %s", cfg.addr)
	}
	if cfg.shutdownTimeout != 10*time.Second {
		t.Errorf("expected shutdownTimeout 10s, got %v", cfg.shutdownTimeout)
	}
}

func TestLoadConfigMissingToken(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestLoadConfigInvalidTimeout(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "mytoken")
	t.Setenv("SHUTDOWN_TIMEOUT", "notaduration")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestLoadConfigAdminSessionSecret(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "mytoken")
	t.Setenv("ADMIN_SESSION_SECRET", "supersecretkey")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cfg.adminSessionSecret, []byte("supersecretkey")) {
		t.Errorf("expected adminSessionSecret %q, got %q", "supersecretkey", cfg.adminSessionSecret)
	}
}

func TestLoadConfigAdminSessionSecretFallback(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "mytoken")
	t.Setenv("ADMIN_SESSION_SECRET", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	derived := sha256.Sum256([]byte("mytoken"))
	if !bytes.Equal(cfg.adminSessionSecret, derived[:]) {
		t.Errorf("expected adminSessionSecret to be sha256 of token, got %x", cfg.adminSessionSecret)
	}
}

func TestLoadConfigAdminCredentialsPartial(t *testing.T) {
	t.Setenv("SHORTENER_TOKEN", "mytoken")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.adminUsername != "admin" {
		t.Errorf("expected adminUsername admin, got %s", cfg.adminUsername)
	}
	if cfg.adminPassword != "" {
		t.Errorf("expected empty adminPassword, got %s", cfg.adminPassword)
	}
}

func TestEndToEndShortenAndResolve(t *testing.T) {
	s, err := store.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	h := handler.New(s)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("POST /shorten", auth.Middleware("test-token", http.HandlerFunc(h.Shorten)))
	mux.HandleFunc("GET /{code}", h.Resolve)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	resp, err := http.Post(ts.URL+"/shorten", "application/json",
		strings.NewReader(`{"url":"https://example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest("POST", ts.URL+"/shorten",
		strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp2, err := client.Get(ts.URL + "/" + result.Code)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("Location") != "https://example.com" {
		t.Errorf("expected Location https://example.com, got %s", resp2.Header.Get("Location"))
	}
}

func TestEndToEndAdminPanel(t *testing.T) {
	s, err := store.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	h := handler.New(s)
	sm := session.New([]byte("test-session-secret"))
	ad, err := admin.New(s, sm, "admin", "password")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	ad.RegisterRoutes(mux)
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("POST /shorten", auth.Middleware("test-token", http.HandlerFunc(h.Shorten)))
	mux.HandleFunc("GET /{code}", h.Resolve)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/admin/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for login page, got %d", resp.StatusCode)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp2, err := client.Get(ts.URL + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 redirect to login, got %d", resp2.StatusCode)
	}

	form := "username=admin&password=password&csrf_token=test"
	req, _ := http.NewRequest("POST", ts.URL+"/admin/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test"})

	resp3, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 after login, got %d", resp3.StatusCode)
	}

	var sessionCookie *http.Cookie
	for _, c := range resp3.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie after login")
	}

	req4, _ := http.NewRequest("GET", ts.URL+"/admin", nil)
	req4.AddCookie(sessionCookie)

	resp4, err := client.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()

	if resp4.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for dashboard with session, got %d", resp4.StatusCode)
	}
}

package main

import (
	"net/http"
	"testing"
	"time"
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
	t.Setenv("DB_PATH", "")
	t.Setenv("PORT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.token != "mytoken" {
		t.Errorf("expected token mytoken, got %s", cfg.token)
	}
	if cfg.dbPath != "shortener.db" {
		t.Errorf("expected dbPath shortener.db, got %s", cfg.dbPath)
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
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("PORT", "9090")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.token != "secret" {
		t.Errorf("expected token secret, got %s", cfg.token)
	}
	if cfg.dbPath != "/tmp/test.db" {
		t.Errorf("expected dbPath /tmp/test.db, got %s", cfg.dbPath)
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

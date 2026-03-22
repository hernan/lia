package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urlshortener/auth"
	"urlshortener/handler"
	"urlshortener/store"
)

type config struct {
	token           string
	dbDriver        string
	dbDsn           string
	addr            string
	shutdownTimeout time.Duration
}

func loadConfig() (config, error) {
	token := os.Getenv("SHORTENER_TOKEN")
	if token == "" {
		return config{}, fmt.Errorf("SHORTENER_TOKEN environment variable is required")
	}

	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "" {
		dbDriver = "sqlite"
	}

	dbDsn := os.Getenv("DB_DSN")
	if dbDsn == "" {
		dbDsn = "shortener.db"
	}

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}

	shutdownTimeout := 5 * time.Second
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		shutdownTimeout = d
	}

	return config{
		token:           token,
		dbDriver:        dbDriver,
		dbDsn:           dbDsn,
		addr:            ":" + addr,
		shutdownTimeout: shutdownTimeout,
	}, nil
}

const (
	defaultReadTimeout  = 5 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

func newServer(cfg config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.addr,
		Handler:      handler,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	s, err := store.Open(cfg.dbDriver, cfg.dbDsn)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	h := handler.New(s)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("POST /shorten", auth.Middleware(cfg.token, http.HandlerFunc(h.Shorten)))
	mux.HandleFunc("GET /{code}", h.Resolve)

	srv := newServer(cfg, mux)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("listening on %s", cfg.addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

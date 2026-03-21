package main

import (
	"context"
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

func main() {
	token := os.Getenv("SHORTENER_TOKEN")
	if token == "" {
		log.Fatal("SHORTENER_TOKEN environment variable is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "shortener.db"
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	h := handler.New(s)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("POST /shorten", auth.Middleware(token, http.HandlerFunc(h.Shorten)))
	mux.HandleFunc("GET /{code}", h.Resolve)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Println("listening on :8080")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

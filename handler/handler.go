package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/mattn/go-sqlite3"

	"urlshortener/shortener"
	"urlshortener/store"
)

const (
	maxRetries  = 5
	maxBodySize = 1 << 20 // 1MB
)

type URLStore interface {
	Create(originalURL, code string) (*store.URL, error)
	GetByCode(code string) (*store.URL, error)
	Ping() error
}

type Handler struct {
	store URLStore
}

func New(s URLStore) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		writeError(w, http.StatusBadRequest, "invalid url")
		return
	}

	var created *store.URL
	for range maxRetries {
		code := shortener.Generate()
		created, err = h.store.Create(req.URL, code)
		if err == nil {
			break
		}
		if !isConstraintError(err) {
			writeError(w, http.StatusInternalServerError, "failed to create short URL")
			return
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create short URL")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"code": created.Code,
		"url":  created.OriginalURL,
	})
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	u, err := h.store.GetByCode(code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, u.OriginalURL, http.StatusMovedPermanently)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint
	}
	return false
}

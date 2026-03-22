package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"urlshortener/internal/httputil"
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
		httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		httputil.WriteError(w, http.StatusBadRequest, "url is required")
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		httputil.WriteError(w, http.StatusBadRequest, "invalid url")
		return
	}

	var created *store.URL
	for range maxRetries {
		code := shortener.Generate()
		created, err = h.store.Create(req.URL, code)
		if err == nil {
			break
		}
		if !errors.Is(err, store.ErrConflict) {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create short URL")
			return
		}
	}
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create short URL")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"code": created.Code,
		"url":  created.OriginalURL,
	})
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		httputil.WriteError(w, http.StatusBadRequest, "code is required")
		return
	}

	u, err := h.store.GetByCode(code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, u.OriginalURL, http.StatusMovedPermanently)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(); err != nil {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

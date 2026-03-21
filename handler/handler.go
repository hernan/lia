package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"urlshortener/shortener"
	"urlshortener/store"
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid url"})
		return
	}

	code := shortener.Generate()
	created, err := h.store.Create(req.URL, code)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create short URL"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"code": created.Code,
		"url":  created.OriginalURL,
	})
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	u, err := h.store.GetByCode(code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

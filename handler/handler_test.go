package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"

	"urlshortener/store"
)

type mockStore struct {
	pingErr error
}

func (m *mockStore) Create(originalURL, code string) (*store.URL, error) {
	return nil, errors.New("not implemented")
}

func (m *mockStore) GetByCode(code string) (*store.URL, error) {
	return nil, errors.New("not implemented")
}

func (m *mockStore) Ping() error {
	return m.pingErr
}

type collisionMockStore struct {
	createCalls int
	failOnce    bool
}

func (m *collisionMockStore) Create(originalURL, code string) (*store.URL, error) {
	m.createCalls++
	if m.failOnce && m.createCalls == 1 {
		return nil, sqlite3.Error{
			Code:         sqlite3.ErrConstraint,
			ExtendedCode: sqlite3.ErrConstraintUnique,
		}
	}
	return &store.URL{
		Code:        code,
		OriginalURL: originalURL,
	}, nil
}

func (m *collisionMockStore) GetByCode(code string) (*store.URL, error) {
	return nil, errors.New("not implemented")
}

func (m *collisionMockStore) Ping() error {
	return nil
}

func newTestHandler(t *testing.T) (*Handler, *httptest.Server) {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	h := New(s)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.Shorten)
	mux.HandleFunc("GET /{code}", h.Resolve)
	mux.HandleFunc("GET /health", h.Health)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return h, ts
}

func TestHealth(t *testing.T) {
	_, ts := newTestHandler(t)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthUnhealthy(t *testing.T) {
	h := &Handler{store: &mockStore{pingErr: errors.New("db down")}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestShortenAndResolve(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":"https://example.com"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
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
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code == "" {
		t.Error("expected non-empty code")
	}
	if result.URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", result.URL)
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

func TestShortenInvalidJSON(t *testing.T) {
	_, ts := newTestHandler(t)

	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShortenEmptyURL(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":""}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShortenInvalidURL(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":"not a url"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShortenURLMissingScheme(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":"example.com"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShortenURLMissingHost(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":"https://"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShortenValidHTTPURL(t *testing.T) {
	_, ts := newTestHandler(t)

	body := `{"url":"http://example.com/path"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestShortenRejectsDangerousSchemes(t *testing.T) {
	_, ts := newTestHandler(t)

	schemes := []string{
		`{"url":"javascript:alert(1)"}`,
		`{"url":"file:///etc/passwd"}`,
		`{"url":"data:text/html,<h1>hi</h1>"}`,
		`{"url":"ftp://example.com/file"}`,
	}

	for _, body := range schemes {
		resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for %s, got %d", body, resp.StatusCode)
		}
	}
}

func TestShortenRetriesOnCollision(t *testing.T) {
	mock := &collisionMockStore{failOnce: true}
	h := New(mock)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.Shorten)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"url":"https://example.com"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if mock.createCalls != 2 {
		t.Errorf("expected 2 create calls (1 fail + 1 success), got %d", mock.createCalls)
	}
}

func TestShortenBodyTooLarge(t *testing.T) {
	_, ts := newTestHandler(t)

	largeBody := `{"url":"` + strings.Repeat("a", maxBodySize+1) + `"}`
	resp, err := http.Post(ts.URL+"/shorten", "application/json", strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestResolveNotFound(t *testing.T) {
	_, ts := newTestHandler(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

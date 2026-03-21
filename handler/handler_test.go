package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"urlshortener/store"
)

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

	// Get the code from the response by reading the Location header
	// Actually, we need to read the JSON response
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

	// Now resolve it — follow redirect manually
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

package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	payload := map[string]any{
		"code": "abc123",
		"url":  "https://example.com",
	}

	WriteJSON(rec, http.StatusCreated, payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusCreated)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want %q", got, "application/json")
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got["code"] != payload["code"] {
		t.Errorf("code = %v, want %v", got["code"], payload["code"])
	}

	if got["url"] != payload["url"] {
		t.Errorf("url = %v, want %v", got["url"], payload["url"])
	}
}

func TestWriteJSONNil(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, http.StatusOK, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Body.String(); got != "null\n" {
		t.Fatalf("body = %q, want %q", got, "null\n")
	}
}

func TestWriteJSONStruct(t *testing.T) {
	rec := httptest.NewRecorder()

	type response struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}

	WriteJSON(rec, http.StatusOK, response{
		Code: "abc123",
		URL:  "https://example.com",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var got response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Code != "abc123" {
		t.Errorf("code = %q, want %q", got.Code, "abc123")
	}

	if got.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", got.URL, "https://example.com")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusBadRequest, "invalid url")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want %q", got, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got["error"] != "invalid url" {
		t.Fatalf("error = %q, want %q", got["error"], "invalid url")
	}
}

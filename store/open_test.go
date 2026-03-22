package store

import (
	"errors"
	"fmt"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenSQLite(t *testing.T) {
	s := newTestStore(t)

	if err := s.Ping(); err != nil {
		t.Errorf("expected Ping to succeed, got %v", err)
	}
}

func TestOpenUnsupportedDriver(t *testing.T) {
	_, err := Open("postgres", "localhost")
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}

func TestCreateAndGetByCode(t *testing.T) {
	s := newTestStore(t)

	created, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Code != "abc123" {
		t.Errorf("expected code abc123, got %s", created.Code)
	}
	if created.OriginalURL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", created.OriginalURL)
	}

	got, err := s.GetByCode("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.OriginalURL != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", got.OriginalURL)
	}
}

func TestGetByCodeNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetByCode("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent code")
	}
}

func TestCreateDuplicateCodeReturnsErrConflict(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Create("https://other.com", "abc123")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestCreateCreatedAtMatchesDB(t *testing.T) {
	s := newTestStore(t)

	created, err := s.Create("https://example.com", "time1")
	if err != nil {
		t.Fatal(err)
	}

	fetched, err := s.GetByCode("time1")
	if err != nil {
		t.Fatal(err)
	}

	if !created.CreatedAt.Equal(fetched.CreatedAt) {
		t.Errorf("returned CreatedAt %v does not match DB CreatedAt %v", created.CreatedAt, fetched.CreatedAt)
	}
}

func TestCreateReturnsPopulatedID(t *testing.T) {
	s := newTestStore(t)

	first, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID <= 0 {
		t.Errorf("expected positive ID, got %d", first.ID)
	}

	second, err := s.Create("https://example.com", "def456")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID <= first.ID {
		t.Errorf("expected second ID (%d) > first ID (%d)", second.ID, first.ID)
	}
}

func TestPingAfterClose(t *testing.T) {
	s, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	if err := s.Ping(); err == nil {
		t.Error("expected non-nil error from Ping on closed store")
	}
}

func seedURLs(t *testing.T, s *Store, n int) []*URL {
	t.Helper()
	urls := make([]*URL, n)
	for i := range n {
		code := fmt.Sprintf("code%d", i)
		u, err := s.Create(fmt.Sprintf("https://example.com/%d", i), code)
		if err != nil {
			t.Fatal(err)
		}
		urls[i] = u
	}
	return urls
}

func TestList(t *testing.T) {
	s := newTestStore(t)
	seedURLs(t, s, 3)

	urls, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(urls))
	}
}

func TestListEmpty(t *testing.T) {
	s := newTestStore(t)

	urls, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(urls))
	}
}

func TestSearch(t *testing.T) {
	s := newTestStore(t)

	s.Create("https://golang.org/pkg", "gopkg")
	s.Create("https://example.com/page", "expage")
	s.Create("https://golang.org/doc", "godoc")

	urls, err := s.Search("golang")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 results, got %d", len(urls))
	}
}

func TestSearchNoResults(t *testing.T) {
	s := newTestStore(t)
	s.Create("https://example.com", "abc123")

	urls, err := s.Search("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 0 {
		t.Errorf("expected 0 results, got %d", len(urls))
	}
}

func TestGetByID(t *testing.T) {
	s := newTestStore(t)
	created, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "abc123" {
		t.Errorf("expected code abc123, got %s", got.Code)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetByID(999)
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestUpdate(t *testing.T) {
	s := newTestStore(t)
	created, err := s.Create("https://old.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Update(created.ID, "https://new.com")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByCode("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.OriginalURL != "https://new.com" {
		t.Errorf("expected https://new.com, got %s", got.OriginalURL)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	created, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Delete(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.GetByCode("abc123")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

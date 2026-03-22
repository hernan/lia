package store

import (
	"errors"
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

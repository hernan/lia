package store

import (
	"errors"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
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

	created, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 0 {
		t.Errorf("expected positive ID, got %d", created.ID)
	}

	second, err := s.Create("https://example.com", "def456")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID <= created.ID {
		t.Errorf("expected second ID (%d) > first ID (%d)", second.ID, created.ID)
	}
}

func TestCreateDuplicateCode(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Create("https://example.com", "abc123")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Create("https://other.com", "abc123")
	if err == nil {
		t.Fatal("expected error for duplicate code")
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

func TestPingHealthy(t *testing.T) {
	s := newTestStore(t)

	if err := s.Ping(); err != nil {
		t.Errorf("expected nil error from Ping on open store, got %v", err)
	}
}

func TestPingAfterClose(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	if err := s.Ping(); err == nil {
		t.Error("expected non-nil error from Ping on closed store")
	}
}

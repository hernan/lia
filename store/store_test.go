package store

import (
	"testing"
	"time"
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

	before := time.Now().UTC()
	created, err := s.Create("https://example.com", "time1")
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()

	fetched, err := s.GetByCode("time1")
	if err != nil {
		t.Fatal(err)
	}

	if created.CreatedAt.Before(before) || created.CreatedAt.After(after) {
		t.Errorf("returned CreatedAt %v not between %v and %v", created.CreatedAt, before, after)
	}

	diff := created.CreatedAt.Sub(fetched.CreatedAt)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("returned CreatedAt %v differs from DB CreatedAt %v by %v", created.CreatedAt, fetched.CreatedAt, diff)
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

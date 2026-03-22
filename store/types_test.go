package store

import (
	"errors"
	"testing"
)

// Compile-time check: verify that a mock type satisfies URLStore.
var _ URLStore = (*mockStore)(nil)

type mockStore struct {
	createErr error
	getByErr  error
	pingErr   error
}

func (m *mockStore) Create(originalURL, code string) (*URL, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &URL{Code: code, OriginalURL: originalURL}, nil
}

func (m *mockStore) GetByCode(code string) (*URL, error) {
	if m.getByErr != nil {
		return nil, m.getByErr
	}
	return &URL{Code: code}, nil
}

func (m *mockStore) GetByID(id int64) (*URL, error) {
	return &URL{ID: id}, nil
}

func (m *mockStore) List() ([]*URL, error) {
	return nil, nil
}

func (m *mockStore) Search(query string) ([]*URL, error) {
	return nil, nil
}

func (m *mockStore) Update(id int64, originalURL string) error {
	return nil
}

func (m *mockStore) Delete(id int64) error {
	return nil
}

func (m *mockStore) Ping() error {
	return m.pingErr
}

func (m *mockStore) Close() error {
	return nil
}

func TestErrConflictSentinel(t *testing.T) {
	wrapped := errors.New("wrapped: " + ErrConflict.Error())
	if errors.Is(wrapped, ErrConflict) {
		t.Error("ErrConflict should not match a wrapped string")
	}

	if !errors.Is(ErrConflict, ErrConflict) {
		t.Error("ErrConflict should match itself")
	}
}

package store

import (
	"errors"
	"time"
)

// ErrConflict is returned by Create when the code already exists in the store.
var ErrConflict = errors.New("code already exists")

// URL represents a shortened URL record.
type URL struct {
	ID          int64
	Code        string
	OriginalURL string
	CreatedAt   time.Time
}

// URLStore defines the interface for URL storage backends.
type URLStore interface {
	Create(originalURL, code string) (*URL, error)
	GetByCode(code string) (*URL, error)
	GetByID(id int64) (*URL, error)
	List() ([]*URL, error)
	Search(query string) ([]*URL, error)
	Update(id int64, originalURL string) error
	Delete(id int64) error
	Ping() error
	Close() error
}

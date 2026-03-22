package store

import (
	"database/sql"
	"fmt"
	"time"

	"urlshortener/store/sqlite"
)

// Store is the concrete storage implementation backed by a SQL database.
type Store struct {
	db     *sql.DB
	driver string
}

// Open connects to the given driver, runs migrations, and returns a *Store.
func Open(driver, dsn string) (*Store, error) {
	var db *sql.DB
	var err error

	switch driver {
	case "sqlite":
		db, err = sqlite.New(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}

	if err := Migrate(db, driver); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db, driver: driver}, nil
}

func (s *Store) Create(originalURL, code string) (*URL, error) {
	now := time.Now().UTC().Truncate(time.Second)
	result, err := s.db.Exec(
		"INSERT INTO urls (code, original_url, created_at) VALUES (?, ?, ?)",
		code, originalURL, now,
	)
	if err != nil {
		if s.isConstraintError(err) {
			return nil, fmt.Errorf("%w", ErrConflict)
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &URL{
		ID:          id,
		Code:        code,
		OriginalURL: originalURL,
		CreatedAt:   now,
	}, nil
}

func (s *Store) GetByCode(code string) (*URL, error) {
	var u URL
	err := s.db.QueryRow(
		"SELECT id, code, original_url, created_at FROM urls WHERE code = ?",
		code,
	).Scan(&u.ID, &u.Code, &u.OriginalURL, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) isConstraintError(err error) bool {
	switch s.driver {
	case "sqlite":
		return sqlite.IsConstraintError(err)
	default:
		return false
	}
}

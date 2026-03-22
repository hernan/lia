package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

// ErrConflict is returned by Create when the code already exists in the store.
var ErrConflict = errors.New("code already exists")

type URL struct {
	ID          int64
	Code        string
	OriginalURL string
	CreatedAt   time.Time
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Create(originalURL, code string) (*URL, error) {
	now := time.Now().UTC().Truncate(time.Second)
	result, err := s.db.Exec(
		"INSERT INTO urls (code, original_url, created_at) VALUES (?, ?, ?)",
		code, originalURL, now,
	)
	if err != nil {
		if isConstraintError(err) {
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

func isConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint
	}
	return false
}

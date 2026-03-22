package store

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateCreatesSchemaTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db, "sqlite"); err != nil {
		t.Fatal(err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatal("schema_migrations table should exist after migrate:", err)
	}
}

func TestMigrateCreatesUrlsTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db, "sqlite"); err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO urls (code, original_url) VALUES (?, ?)", "test", "https://example.com")
	if err != nil {
		t.Fatal("urls table should exist after migrate:", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db, "sqlite"); err != nil {
		t.Fatal("second migrate should not fail:", err)
	}
}

func TestMigrateUnsupportedDriver(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	err = Migrate(db, "postgres")
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}

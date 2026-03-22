package sqlite

import (
	"testing"

	"github.com/mattn/go-sqlite3"
)

func TestNewInMemory(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Errorf("expected Ping to succeed on fresh in-memory DB, got %v", err)
	}
}

func TestIsConstraintError(t *testing.T) {
	constraintErr := sqlite3.Error{
		Code: sqlite3.ErrConstraint,
	}
	if !IsConstraintError(constraintErr) {
		t.Error("expected IsConstraintError to return true for constraint error")
	}

	otherErr := sqlite3.Error{
		Code: sqlite3.ErrBusy,
	}
	if IsConstraintError(otherErr) {
		t.Error("expected IsConstraintError to return false for non-constraint error")
	}

	if IsConstraintError(nil) {
		t.Error("expected IsConstraintError to return false for nil")
	}
}

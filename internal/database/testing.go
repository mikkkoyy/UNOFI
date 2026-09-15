package database

import (
	"database/sql"
	"testing"
)

// TestDB creates an in-memory database for testing.
func TestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:", 10000)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Tx executes a function within a transaction and rolls back on error.
func Tx(db *DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

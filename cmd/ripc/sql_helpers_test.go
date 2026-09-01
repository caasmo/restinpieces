package main

import (
	"database/sql"
	"testing"

	dbm "github.com/caasmo/restinpieces/db/modernc"
)

// newTestAppDb opens an in-memory SQLite connection for testing and wires
// it into an appDb. The database is never closed here: tests that need a
// closed database call db.Close() themselves, and in-memory databases are
// reclaimed when the test process exits.
func newTestAppDb(t *testing.T) *appDb {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	// One connection: an in-memory database lives per connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	mdb, err := dbm.New(db)
	if err != nil {
		t.Fatalf("failed to instantiate modernc db from connection: %v", err)
	}

	return &appDb{db: db, Db: mdb}
}

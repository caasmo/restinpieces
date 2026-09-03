package databasesql

import (
	"database/sql"
	"io/fs"
	"testing"

	sqlfs "github.com/caasmo/restinpieces/sql"
)

func newTestDb(t *testing.T, schemaPaths ...string) *Db {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}
	// One connection: an in-memory database lives per connection, so the
	// pool must never grow beyond the single connection that owns it.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	schemaFS := sqlfs.FS()
	for _, p := range schemaPaths {
		sqlBytes, err := fs.ReadFile(schemaFS, p)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", p, err)
		}

		_, err = db.Exec(string(sqlBytes))
		if err != nil {
			t.Fatalf("Failed to execute %s: %v", p, err)
		}
	}

	mdb, err := New(db)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return mdb
}

// registerUsersStmts registers the users read statements on mdb,
// mirroring what restinpieces.WithModerncPool does for the app.
func registerUsersStmts(t testing.TB, mdb *Db) {
	t.Helper()

	for name, sql := range UsersStmts {
		if _, err := mdb.RegisterStmt(name, sql); err != nil {
			t.Fatalf("failed to register statement: %v", err)
		}
	}
}

// registerQueueStmts registers the queue statements on mdb,
// mirroring what restinpieces.WithModerncPool does for the app.
func registerQueueStmts(t testing.TB, mdb *Db) {
	t.Helper()

	for name, sql := range QueueStmts {
		if _, err := mdb.RegisterStmt(name, sql); err != nil {
			t.Fatalf("failed to register statement: %v", err)
		}
	}
}

package main

import (
	"testing"

	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestAppDb opens an in-memory SQLite connection pool for testing and wires
// it into an appDb. The pool is never closed here: tests that need a closed
// pool call db.Close() themselves, and in-memory pools are reclaimed when the
// test process exits.
func newTestAppDb(t *testing.T) *appDb {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	zdb, err := dbz.New(pool)
	if err != nil {
		t.Fatalf("failed to instantiate zombiezen db from pool: %v", err)
	}

	return &appDb{pool: pool, Db: zdb}
}

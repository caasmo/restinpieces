package zombiezen

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newBenchDb creates a production-like database for benchmarks.
// Unlike newTestDb, it uses a real temp file instead of an in-memory
// database, so multiple pool connections share the same data. The pool is
// built through NewPool, which mirrors production settings (WAL mode,
// busy_timeout, one connection per CPU). The temp file and pool are cleaned
// up automatically when the benchmark finishes.
func newBenchDb(b *testing.B, schemaPaths ...string) *Db {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "bench.db")
	pool, err := NewPool(dbPath)
	if err != nil {
		b.Fatalf("failed to create bench pool: %v", err)
	}
	b.Cleanup(func() {
		if err := pool.Close(); err != nil {
			b.Errorf("failed to close bench pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		b.Fatalf("failed to get bench connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := sql.FS()
	for _, p := range schemaPaths {
		sqlBytes, err := fs.ReadFile(schemaFS, p)
		if err != nil {
			b.Fatalf("failed to read %s: %v", p, err)
		}

		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			b.Fatalf("failed to execute %s: %v", p, err)
		}
	}

	db, err := New(pool)
	if err != nil {
		b.Fatalf("failed to create bench db: %v", err)
	}
	return db
}

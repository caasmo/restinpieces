package zombiezen

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite"
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

// newBenchLog creates a log database on a real temp file, mirroring
// newBenchDb for the Log type. Log uses a single connection, not a pool, so
// it needs its own setup. The temp file and connection are cleaned up
// automatically when the benchmark finishes.
func newBenchLog(b *testing.B) *Log {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "bench_log.db")
	dsn := fmt.Sprintf("file:%s", dbPath)

	conn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenURI)
	if err != nil {
		b.Fatalf("failed to create db conn for schema setup: %v", err)
	}

	schemaFS := sql.FS()
	sqlBytes, err := fs.ReadFile(schemaFS, "log/logs.sql")
	if err != nil {
		b.Fatalf("failed to read log/logs.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		b.Fatalf("failed to execute logs.sql script: %v", err)
	}
	if err := conn.Close(); err != nil {
		b.Fatalf("failed to close setup connection: %v", err)
	}

	logConn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenURI)
	if err != nil {
		b.Fatalf("failed to open log conn: %v", err)
	}
	logDB, err := NewLog(logConn)
	if err != nil {
		b.Fatalf("failed to create new log db: %v", err)
	}
	b.Cleanup(func() {
		if err := logDB.Close(); err != nil && err != ErrConnectionClosed {
			b.Errorf("failed to close log db: %v", err)
		}
	})

	return logDB
}

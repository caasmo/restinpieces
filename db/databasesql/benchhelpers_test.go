package databasesql

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/sql"
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

	schemaFS := sql.FS()
	for _, p := range schemaPaths {
		sqlBytes, err := fs.ReadFile(schemaFS, p)
		if err != nil {
			b.Fatalf("failed to read %s: %v", p, err)
		}

		_, err = pool.Exec(string(sqlBytes))
		if err != nil {
			b.Fatalf("failed to execute %s: %v", p, err)
		}
	}

	mdb, err := New(pool)
	if err != nil {
		b.Fatalf("failed to create bench db: %v", err)
	}
	return mdb
}

// newBenchLog creates a log database on a real temp file, mirroring
// newBenchDb for the Log type. Log uses a single connection, not a pool, so
// it needs its own setup. batchSize is the size the Log is built for,
// matching the benchmark sub-run. The temp file and connection are cleaned
// up automatically when the benchmark finishes.
func newBenchLog(b *testing.B, batchSize int) *Log {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "bench_log.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		b.Fatalf("failed to create bench log file: %v", err)
	}

	// Schema setup goes through the production single-connection
	// constructor so the file is born with the same pragmas (WAL,
	// synchronous=NORMAL) as in production.
	sqlDB, err := NewConn(dbPath)
	if err != nil {
		b.Fatalf("failed to open log conn: %v", err)
	}

	sqlBytes, err := fs.ReadFile(sql.FS(), "log/logs.sql")
	if err != nil {
		b.Fatalf("failed to read log/logs.sql: %v", err)
	}
	_, err = sqlDB.Exec(string(sqlBytes))
	if err != nil {
		b.Fatalf("failed to execute logs.sql script: %v", err)
	}

	logDB, err := NewLog(sqlDB, batchSize)
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

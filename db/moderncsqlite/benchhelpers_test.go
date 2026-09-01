package moderncsqlite

import (
	"context"
	"database/sql/driver"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/sql"
)

// newBenchLog creates a log database on a real temp file for benchmarks,
// mirroring db/databasesql's newBenchLog: same pragmas (WAL, synchronous
// NORMAL), single connection, schema from log/logs.sql. entries is the batch
// size the Log is built for, matching the benchmark sub-run. The temp file
// and connection are cleaned up automatically when the benchmark finishes.
func newBenchLog(b *testing.B, entries int) *Log {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "bench_log.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		b.Fatalf("failed to create bench log file: %v", err)
	}

	conn, err := NewConn(buildLogDSN(dbPath))
	if err != nil {
		b.Fatalf("failed to open log conn: %v", err)
	}

	exec := conn.(driver.ExecerContext)
	sqlBytes, err := fs.ReadFile(sql.FS(), "log/logs.sql")
	if err != nil {
		b.Fatalf("failed to read log/logs.sql: %v", err)
	}
	if _, err := exec.ExecContext(context.Background(), string(sqlBytes), nil); err != nil {
		b.Fatalf("failed to execute logs.sql script: %v", err)
	}

	logConn, err := NewLog(conn, entries)
	if err != nil {
		b.Fatalf("failed to create new log db: %v", err)
	}

	b.Cleanup(func() {
		if err := logConn.Close(); err != nil && err != ErrConnectionClosed {
			b.Errorf("failed to close log db: %v", err)
		}
	})

	return logConn
}

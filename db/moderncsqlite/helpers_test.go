package moderncsqlite

import (
	"context"
	"database/sql/driver"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/sql"
)

// buildLogDSN returns the modernc DSN for a log database file with the same
// pragmas as db/modernc's defaultPragmas, so tests and benchmarks compare
// like for like: busy_timeout 5000, WAL, synchronous NORMAL, foreign_keys
// off, and mode=rw (the file must already exist).
func buildLogDSN(dbPath string) string {
	pragmas := []string{
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(off)",
	}
	return "file:" + dbPath + "?" + strings.Join(pragmas, "&") + "&mode=rw"
}

// newTestLogDB creates a new temporary SQLite database, applies the logs
// schema, and returns an initialized *Log object for testing, along with
// the db path. NewConn requires the file to pre-exist (mode=rw), so it is
// created empty first.
func newTestLogDB(t *testing.T) (*Log, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_log.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}

	conn, err := NewConn(buildLogDSN(dbPath))
	if err != nil {
		t.Fatalf("failed to open log conn: %v", err)
	}

	exec := conn.(driver.ExecerContext)
	sqlBytes, err := fs.ReadFile(sql.FS(), "log/logs.sql")
	if err != nil {
		t.Fatalf("failed to read log/logs.sql: %v", err)
	}
	if _, err := exec.ExecContext(context.Background(), string(sqlBytes), nil); err != nil {
		t.Fatalf("failed to execute logs.sql script: %v", err)
	}

	logConn, err := NewLog(conn, 100)
	if err != nil {
		t.Fatalf("failed to create new log db: %v", err)
	}

	t.Cleanup(func() {
		if err := logConn.Close(); err != nil && err != ErrConnectionClosed {
			t.Errorf("failed to close log db: %v", err)
		}
	})

	return logConn, dbPath
}

package databasesql

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/dbtest"
	"github.com/caasmo/restinpieces/sql"
)

// newTestLogDB creates a new temporary SQLite database, applies the logs
// schema, and returns an initialized *Log object for testing, along with
// the db path. NewConn requires the file to pre-exist, so it is created
// empty first.
func newTestLogDB(t *testing.T) (*Log, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_log.db")
	err := os.WriteFile(dbPath, nil, 0o600)
	if err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}

	logConn, err := NewConn(dbPath)
	if err != nil {
		t.Fatalf("failed to open log conn: %v", err)
	}

	schemaFS := sql.FS()
	sqlBytes, err := fs.ReadFile(schemaFS, "log/logs.sql")
	if err != nil {
		t.Fatalf("Failed to read log/logs.sql: %v", err)
	}

	_, err = logConn.Exec(string(sqlBytes))
	if err != nil {
		t.Fatalf("Failed to execute logs.sql script: %v", err)
	}

	logDB, err := NewLog(logConn, 100)
	if err != nil {
		t.Fatalf("failed to create new log db: %v", err)
	}

	t.Cleanup(func() {
		if err := logDB.Close(); err != nil && err != ErrConnectionClosed {
			t.Errorf("failed to close log db: %v", err)
		}
	})

	return logDB, dbPath
}

func TestLogSuite(t *testing.T) {
	dbtest.LogSuite{New: func(t *testing.T) db.DbLog {
		logDB, _ := newTestLogDB(t)
		return logDB
	}}.RunAll(t)
}

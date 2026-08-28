package zombiezen

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/dbtest"
	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestLogDB creates a new temporary SQLite database, applies the logs schema,
// and returns an initialized *Log object for testing, along with the db path.
func newTestLogDB(t *testing.T) (*Log, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_log.db")

	// Apply the logs schema using a temporary connection
	dsn := fmt.Sprintf("file:%s", dbPath)
	conn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenURI)
	if err != nil {
		t.Fatalf("failed to create db conn for schema setup: %v", err)
	}

	schemaFS := sql.FS()
	sqlBytes, err := fs.ReadFile(schemaFS, "log/logs.sql")
	if err != nil {
		t.Fatalf("Failed to read log/logs.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute logs.sql script: %v", err)
	}
	// Close the temporary connection used for setup
	if err := conn.Close(); err != nil {
		t.Fatalf("Failed to close setup connection: %v", err)
	}

	// Create the Log object for the test with a new connection
	logConn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenURI)
	if err != nil {
		t.Fatalf("failed to open log conn: %v", err)
	}
	logDB, err := NewLog(logConn)
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


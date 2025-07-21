package zombiezen

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/migrations"
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
	conn, err := NewConn(dbPath)
	if err != nil {
		t.Fatalf("failed to create db conn for schema setup: %v", err)
	}

	schemaFS := migrations.Schema()
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

	// Create the Log object for the test, which opens its own connection
	logDB, err := NewLog(dbPath)
	if err != nil {
		t.Fatalf("failed to create new log db: %v", err)
	}

	t.Cleanup(func() {
		logDB.Close()
	})

	return logDB, dbPath
}

// getLogRecordCount is a helper to count records in the logs table.
func getLogRecordCount(t *testing.T, logDB *Log) int {
	t.Helper()
	var count int
	err := sqlitex.Execute(logDB.conn, "SELECT COUNT(*) FROM logs", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = int(stmt.ColumnInt64(0))
			return nil
		},
	})
	if err != nil {
		t.Fatalf("failed to count log records: %v", err)
	}
	return count
}

func TestLog_InsertBatch(t *testing.T) {
	t.Run("Successful Batch Insert", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		batch := []db.Log{
			{Level: 1, Message: "message 1", JsonData: `{"key":"value1"}`, Created: db.TimeFormat(time.Now())},
			{Level: 2, Message: "message 2", JsonData: `{"key":"value2"}`, Created: db.TimeFormat(time.Now())},
		}

		err := logDB.InsertBatch(batch)
		if err != nil {
			t.Fatalf("InsertBatch() failed: %v", err)
		}

		count := getLogRecordCount(t, logDB)
		if count != len(batch) {
			t.Errorf("expected %d records, got %d", len(batch), count)
		}
	})

	t.Run("Empty Batch", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		err := logDB.InsertBatch([]db.Log{})
		if err != nil {
			t.Fatalf("InsertBatch() with empty slice failed: %v", err)
		}
		count := getLogRecordCount(t, logDB)
		if count != 0 {
			t.Errorf("expected 0 records for empty batch, got %d", count)
		}
	})

	t.Run("Single Entry Batch", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		batch := []db.Log{
			{Level: 1, Message: "single message", JsonData: "{}", Created: db.TimeFormat(time.Now())},
		}
		err := logDB.InsertBatch(batch)
		if err != nil {
			t.Fatalf("InsertBatch() with single entry failed: %v", err)
		}
		count := getLogRecordCount(t, logDB)
		if count != 1 {
			t.Errorf("expected 1 record for single entry batch, got %d", count)
		}
	})

	t.Run("Rollback on Failure", func(t *testing.T) {
		logDB, dbPath := newTestLogDB(t)

		// Close the connection to force a failure on the next operation
		logDB.Close()

		batch := []db.Log{
			{Level: 1, Message: "this should fail", JsonData: "{}", Created: db.TimeFormat(time.Now())},
		}
		err := logDB.InsertBatch(batch)
		if err == nil {
			t.Fatal("InsertBatch() should have failed on a closed connection")
		}

		// Re-open a new connection to the same database file to verify its state
		verifyConn, err := NewConn(dbPath)
		if err != nil {
			t.Fatalf("failed to reopen db for verification: %v", err)
		}
		defer verifyConn.Close()

		var count int
		err = sqlitex.Execute(verifyConn, "SELECT COUNT(*) FROM logs", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = int(stmt.ColumnInt64(0))
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to count records for verification: %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 records after a failed batch insert, but found %d", count)
		}
	})
}

func TestLog_Ping(t *testing.T) {
	logDB, _ := newTestLogDB(t)

	testCases := []struct {
		name        string
		tableName   string
		expectErr   bool
		errContains string
	}{
		{"ValidTable", "logs", false, ""},
		{"NonExistentTable", "non_existent_table", true, "no such table: non_existent_table"},
		{"SystemTable", "sqlite_master", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := logDB.Ping(tc.tableName)
			if (err != nil) != tc.expectErr {
				t.Fatalf("Ping() error = %v, expectErr %v", err, tc.expectErr)
			}
			if tc.expectErr && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("Ping() error message = %q, want to contain %q", err.Error(), tc.errContains)
			}
		})
	}
}

func TestLog_Close(t *testing.T) {
	t.Run("Successful Close", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		err := logDB.Close()
		if err != nil {
			t.Errorf("Close() returned an unexpected error: %v", err)
		}
	})

	t.Run("Double Close", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		err := logDB.Close()
		if err != nil {
			t.Fatalf("first Close() failed unexpectedly: %v", err)
		}
		err = logDB.Close()
		if err != ErrConnectionClosed {
			t.Errorf("second Close() should have returned ErrConnectionClosed, but got: %v", err)
		}
	})

	t.Run("Operations After Close", func(t *testing.T) {
		logDB, _ := newTestLogDB(t)
		logDB.Close()

		err := logDB.Ping("logs")
		if err != ErrConnectionClosed {
			t.Errorf("Ping() after Close() should have returned ErrConnectionClosed, but got: %v", err)
		}

		err = logDB.InsertBatch([]db.Log{{}})
		if err != ErrConnectionClosed {
			t.Errorf("InsertBatch() after Close() should have returned ErrConnectionClosed, but got: %v", err)
		}
	})
}

package zombiezen

import (
	"fmt"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// DBLogEntry holds pre-processed log data ready for DB insertion.
type DBLogEntry struct {
	level    int64
	message  string
	jsonData string // Pre-marshalled JSON for the 'data' field
	created  string // Pre-formatted time string for 'created'
}

// NewConn creates a new SQLite connection for logging purposes.
func NewConn(dbPath string) (*sqlite.Conn, error) {
	return sqlite.OpenConn(dbPath, sqlite.OpenReadWrite|sqlite.OpenCreate)
}

// WriteLogBatch writes a batch of log entries to the SQLite database.
func WriteLogBatch(conn *sqlite.Conn, batch []DBLogEntry) error {
	if len(batch) == 0 {
		return nil
	}

	err := sqlitex.Execute(conn, "BEGIN;", nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := conn.Prepare("INSERT INTO _logs (level, message, data, created) VALUES ($level, $message, $data, $created)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Finalize()

	for _, entry := range batch {
		stmt.SetInt64("$level", entry.level)
		stmt.SetText("$message", entry.message)
		stmt.SetText("$data", entry.jsonData)
		stmt.SetText("$created", entry.created)

		if _, err := stmt.Step(); err != nil {
			stmt.Reset()
			return fmt.Errorf("failed to execute statement for record (msg: %q): %w", entry.message, err)
		}
		stmt.Reset()
	}

	err = sqlitex.Execute(conn, "COMMIT;", nil)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)


// NewConn creates a new SQLite connection for logging purposes.
func NewConn(dbPath string) (*sqlite.Conn, error) {
	return sqlite.OpenConn(dbPath, sqlite.OpenReadWrite|sqlite.OpenCreate)
}

// WriteLogBatch writes a batch of log entries to the SQLite database.
func WriteLogBatch(conn *sqlite.Conn, batch []db.Log) error {
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
		stmt.SetInt64("$level", entry.Level)
		stmt.SetText("$message", entry.Message)
		stmt.SetText("$data", entry.JsonData)
		stmt.SetText("$created", entry.Created)

		if _, err := stmt.Step(); err != nil {
			stmt.Reset()
			return fmt.Errorf("failed to execute statement for record (msg: %q): %w", entry.Message, err)
		}
		stmt.Reset()
	}

	err = sqlitex.Execute(conn, "COMMIT;", nil)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

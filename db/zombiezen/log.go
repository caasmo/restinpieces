package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var ErrConnectionClosed = fmt.Errorf("database connection is closed")

// Verify interface implementation
var _ db.DbLog = (*Log)(nil)

// Log represents a connection to the SQLite database for logging purposes.
type Log struct {
	conn *sqlite.Conn
}

// NewLog creates a Log from an existing connection.
func NewLog(conn *sqlite.Conn) (*Log, error) {
	if conn == nil {
		return nil, fmt.Errorf("conn cannot be nil")
	}
	return &Log{conn: conn}, nil
}

// InsertBatch writes a batch of log entries to the SQLite database.
// It uses an explicit transaction that will be rolled back on any error.
func (l *Log) InsertBatch(batch []db.Log) (err error) {
	if l.conn == nil {
		return ErrConnectionClosed
	}
	if len(batch) == 0 {
		return nil
	}

	// Start immediate transaction for better concurrency control
	if err = sqlitex.Execute(l.conn, "BEGIN IMMEDIATE;", nil); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case we exit early
	defer func() {
		if err != nil {
			_ = sqlitex.Execute(l.conn, "ROLLBACK;", nil)
		}
	}()

	// Prepare insert statement
	var stmt *sqlite.Stmt
	stmt, err = l.conn.Prepare("INSERT INTO logs (level, message, data, created) VALUES ($level, $message, $data, $created)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil && err == nil {
			err = fmt.Errorf("failed to finalize statement: %w", ferr)
		}
	}()

	// Insert each record
	for _, entry := range batch {
		stmt.SetInt64("$level", entry.Level)
		stmt.SetText("$message", entry.Message)
		stmt.SetText("$data", entry.JsonData)
		stmt.SetText("$created", entry.Created)

		if _, err = stmt.Step(); err != nil {
			_ = stmt.Reset()
			err = fmt.Errorf("failed to execute statement for record (msg: %q): %w", entry.Message, err)
			return err
		}

		if err = stmt.Reset(); err != nil {
			err = fmt.Errorf("failed to reset statement: %w", err)
			return err
		}
	}

	// Commit transaction if all inserts succeeded
	if err = sqlitex.Execute(l.conn, "COMMIT;", nil); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the underlying SQLite connection.
func (l *Log) Close() error {
	if l.conn == nil {
		return ErrConnectionClosed
	}
	err := l.conn.Close()
	l.conn = nil // Set to nil after closing
	return err
}

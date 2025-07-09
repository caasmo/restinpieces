package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Verify interface implementation
var _ db.DbLog = (*Log)(nil)

// Log represents a connection to the SQLite database for logging purposes.
type Log struct {
	conn *sqlite.Conn
}

// New creates a new SQLite connection for logging purposes with performance optimizations.
// It returns a pointer to a Log struct.
func NewLog(dbPath string) (*Log, error) {
	conn, err := NewConn(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open logging connection: %w", err)
	}

	return &Log{conn: conn}, nil
}

// InsertBatch writes a batch of log entries to the SQLite database.
// It uses an explicit transaction that will be rolled back on any error.
func (l *Log) InsertBatch(batch []db.Log) (err error) {
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

// Ping checks if the specified table exists.
func (l *Log) Ping(tableName string) (err error) {
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", tableName)
	var stmt *sqlite.Stmt
	stmt, _, err = l.conn.PrepareTransient(query)
	if err != nil {
		return fmt.Errorf("failed to prepare ping statement for table %s: %w", tableName, err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil && err == nil {
			err = ferr
		}
	}()

	if _, err = stmt.Step(); err != nil {
		// Check if the error is due to a missing table
		if sqlite.ErrCode(err) == sqlite.ResultError {
			return fmt.Errorf("table '%s' not found: %w", tableName, err)
		}
		return fmt.Errorf("failed to execute ping statement for table %s: %w", tableName, err)
	}

	return nil
}

// Close closes the underlying SQLite connection.
func (l *Log) Close() error {
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}

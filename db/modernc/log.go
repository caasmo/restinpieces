package modernc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

var ErrConnectionClosed = fmt.Errorf("database connection is closed")

// Verify interface implementation
var _ db.DbLog = (*Log)(nil)

// Log represents a connection to the SQLite database for logging purposes.
type Log struct {
	db *sql.DB
}

// NewLog creates a Log from an existing database connection.
// It validates that the required schema exists before returning.
func NewLog(sqlDB *sql.DB) (*Log, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("db cannot be nil")
	}
	l := &Log{db: sqlDB}
	if err := l.Ping("logs"); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}
	return l, nil
}

// InsertBatch writes a batch of log entries to the SQLite database.
// It uses an explicit transaction that will be rolled back on any error.
// The transaction begins with BEGIN IMMEDIATE: a RESERVED lock is acquired
// immediately for better concurrency control, matching the previous driver.
func (l *Log) InsertBatch(batch []db.Log) (err error) {
	if l.db == nil {
		return ErrConnectionClosed
	}
	if len(batch) == 0 {
		return nil
	}

	// Pin one connection: all transaction statements must run on it.
	conn, err := l.db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get connection for batch insert: %w", err)
	}
	defer func() {
		closeErr := conn.Close()
		err = errors.Join(err, closeErr)
	}()

	// Begin an immediate transaction for better concurrency control
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE;"); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case we exit early
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK;")
		}
	}()

	// Prepare insert statement
	stmt, err := conn.PrepareContext(context.Background(), "INSERT INTO logs (level, message, data, created) VALUES ($level, $message, $data, $created)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if ferr := stmt.Close(); ferr != nil && err == nil {
			err = fmt.Errorf("failed to close statement: %w", ferr)
		}
	}()

	// Insert each record
	for _, entry := range batch {
		if _, err = stmt.ExecContext(context.Background(),
			sql.Named("level", entry.Level),
			sql.Named("message", entry.Message),
			sql.Named("data", entry.JsonData),
			sql.Named("created", entry.Created)); err != nil {
			return fmt.Errorf("failed to execute statement for record (msg: %q): %w", entry.Message, err)
		}
	}

	// Commit transaction if all inserts succeeded
	if _, err = conn.ExecContext(context.Background(), "COMMIT;"); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Ping checks if the specified table exists.
func (l *Log) Ping(tableName string) (err error) {
	if l.db == nil {
		return ErrConnectionClosed
	}
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", tableName)
	rows, err := l.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to ping table %s: %w", tableName, err)
	}
	return rows.Close()
}

// Close closes the underlying SQLite database.
func (l *Log) Close() error {
	if l.db == nil {
		return ErrConnectionClosed
	}
	err := l.db.Close()
	l.db = nil // Set to nil after closing
	return err
}

package modernc

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

// InsertBatch writes a batch of log entries to the SQLite database as a
// single multi-row INSERT statement. One statement is atomic in SQLite:
// the whole batch is committed or rolled back as a unit, so no explicit
// transaction is needed.
//
// The statement uses 4 bind parameters per entry. SQLite rejects a single
// statement with more than SQLITE_MAX_VARIABLE_NUMBER bind parameters
// (default 32766) — at 4 parameters per entry that is 8191 entries.
// Batches must stay below that ceiling or the statement fails with
// "too many SQL variables"; the framework's default flush size is 100.
func (l *Log) InsertBatch(batch []db.Log) error {
	if l.db == nil {
		return ErrConnectionClosed
	}
	if len(batch) == 0 {
		return nil
	}

	args := make([]any, 0, len(batch)*4)
	for _, entry := range batch {
		args = append(args, entry.Level, entry.Message, entry.JsonData, entry.Created)
	}

	if _, err := l.db.ExecContext(context.Background(), buildLogInsertStatement(len(batch)), args...); err != nil {
		return fmt.Errorf("failed to execute batch insert statement: %w", err)
	}
	return nil
}

// buildLogInsertStatement returns the multi-row INSERT statement for
// entries value tuples:
//
//	INSERT INTO logs (level, message, data, created) VALUES (?,?,?,?), (?,?,?,?), ...
//
// Positional '?' placeholders avoid the per-call allocation of named
// parameters, and a single statement performs one argument conversion
// round for the whole batch.
func buildLogInsertStatement(entries int) string {
	var sb strings.Builder
	sb.Grow(56 + entries*11) // column list plus one tuple and separator per entry
	sb.WriteString("INSERT INTO logs (level, message, data, created) VALUES ")
	for i := 0; i < entries; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?,?,?,?)")
	}
	return sb.String()
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

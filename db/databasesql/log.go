package databasesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/caasmo/restinpieces/db"
)

var ErrConnectionClosed = fmt.Errorf("database connection is closed")

// Verify interface implementation
var _ db.DbLog = (*Log)(nil)

// Log represents a connection to the SQLite database for logging purposes,
// opened through the database/sql stdlib layer on a single-connection pool.
//
// A Log must be used from a single goroutine: the arg slice and prepared
// statement are reused across InsertBatch calls, so concurrent flushes
// would overwrite each other's arguments.
type Log struct {
	db        *sql.DB   // for Close and the partial-flush path
	args      []any     // pooled bind args; values rewritten per flush
	stmt      *sql.Stmt // multi-row INSERT for batchSize entries
	batchSize int       // configured batch size; only batches of this size use stmt
}

// NewLog wraps a single-connection *sql.DB (as returned by NewConn). It
// validates that the required schema exists and prepares the multi-row
// INSERT for batchSize entries, so InsertBatch does no per-flush string
// building or prepare; the statement is never re-built or re-prepared.
func NewLog(sqlDB *sql.DB, batchSize int) (*Log, error) {
	l := &Log{db: sqlDB, batchSize: batchSize}
	if err := l.Ping("logs"); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	stmt, err := sqlDB.Prepare(buildLogInsertStatement(batchSize))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare batch insert statement: %w", err)
	}
	l.stmt = stmt
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
//
// The multi-row INSERT is built and prepared in NewLog for the configured
// batch size and reused; it is never re-built or re-prepared. A partial
// flush (e.g. ticker or shutdown drain) is executed once through the
// database, which prepares and closes the statement internally, leaving
// the prepared statement untouched.
func (l *Log) InsertBatch(batch []db.Log) error {
	if l.db == nil {
		return ErrConnectionClosed
	}
	if len(batch) == 0 {
		return nil
	}

	l.bindArgs(batch)

	if len(batch) != l.batchSize {
		_, err := l.db.ExecContext(context.Background(), buildLogInsertStatement(len(batch)), l.args...)
		if err != nil {
			return fmt.Errorf("failed to execute partial insert statement: %w", err)
		}
		return nil
	}

	_, err := l.stmt.ExecContext(context.Background(), l.args...)
	if err != nil {
		return fmt.Errorf("failed to execute batch insert statement: %w", err)
	}
	return nil
}

// bindArgs fills l.args with the 4 bind arguments of each entry. The backing
// array is pooled across flushes and only grown when needed; per flush only
// the values are rewritten. This is safe because database/sql converts the
// arguments and the driver binds them into SQLite memory during
// ExecContext, so nothing retains the slice after the call returns.
func (l *Log) bindArgs(batch []db.Log) {
	if cap(l.args) < len(batch)*4 {
		l.args = make([]any, len(batch)*4)
	}
	l.args = l.args[:len(batch)*4]
	for i, e := range batch {
		base := i * 4
		l.args[base] = e.Level
		l.args[base+1] = e.Message
		l.args[base+2] = e.JsonData
		l.args[base+3] = e.Created
	}
}

// Fragments of the multi-row INSERT built in NewLog and the partial flush
// path. Sizes come from len(), so the Grow hint can never drift from the
// actual strings.
const (
	insertPrefix = "INSERT INTO logs (level, message, data, created) VALUES "
	insertTuple  = "(?,?,?,?)"
	insertSep    = ", "
)

// buildLogInsertStatement returns the multi-row INSERT for entries value
// tuples:
//
//	INSERT INTO logs (level, message, data, created) VALUES (?,?,?,?), (?,?,?,?), ...
//
// Positional '?' placeholders keep the stmt preparable with a single
// argument conversion round for the whole batch.
func buildLogInsertStatement(entries int) string {
	var sb strings.Builder
	sb.Grow(len(insertPrefix) + entries*(len(insertTuple)+len(insertSep)))
	sb.WriteString(insertPrefix)
	for i := 0; i < entries; i++ {
		if i > 0 {
			sb.WriteString(insertSep)
		}
		sb.WriteString(insertTuple)
	}
	return sb.String()
}

// Ping checks if the specified table exists.
func (l *Log) Ping(tableName string) error {
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

// Close closes the underlying SQLite database and the prepared statement.
func (l *Log) Close() error {
	if l.db == nil {
		return ErrConnectionClosed
	}
	stmtErr := l.stmt.Close()
	dbErr := l.db.Close()
	l.db = nil // Set to nil after closing
	l.stmt = nil
	l.args = nil
	return errors.Join(stmtErr, dbErr)
}

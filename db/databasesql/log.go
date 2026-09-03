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

// Log writes log batches to SQLite on a single-connection *sql.DB pool
// (NewConn caps the pool at one). InsertBatch is the hot path, made lean by
// two tricks: stmt is prepared once for batchSize and reused (no Prepare or
// string building per flush), and args is pooled and rewritten in place per
// flush (no allocation per flush from our code). Both force single-goroutine
// use; the log daemon is the only caller.
//
// Performance (per InsertBatch of 100 entries, benchtime=100x):
// 29.5 KiB/op, 315 allocs/op (this type).
//
// database/sql repacks the batch from our []any into a fresh
// []driver.NamedValue per flush — an unbox/convert/re-box hop per value.
// The remaining allocs are the modernc driver binding the 4xN parameters.
type Log struct {
	db   *sql.DB   // pool; also serves the partial-flush path
	stmt *sql.Stmt // prepared multi-row INSERT for batchSize entries
	args []any     // pooled bind args; rewritten per flush
	// TODO: batchSize mirrors config log.batch.batch_size, read once at
	// startup. A config change requires re-preparing stmt for the new size;
	// Log is not rebuilt on SIGHUP reload today.
	batchSize int // configured batch size; only full batches use stmt
}

// NewLog wraps a single-connection *sql.DB (as returned by NewConn). It
// prepares the multi-row INSERT for batchSize entries, so InsertBatch does
// no per-flush string building or prepare; the statement is never re-built
// or re-prepared. The prepare itself validates the schema: it fails when
// the logs table or one of its columns is missing.
func NewLog(sqlDB *sql.DB, batchSize int) (*Log, error) {
	l := &Log{db: sqlDB, batchSize: batchSize}

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
// "too many SQL variables"; the framework's default batch size is 50.
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

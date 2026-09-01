package moderncsqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/caasmo/restinpieces/db"
)

var ErrConnectionClosed = fmt.Errorf("database connection is closed")

// Log represents a connection to the SQLite database for logging purposes,
// opened directly through the modernc driver, bypassing the database/sql
// stdlib layer.
//
// A Log must be used from a single goroutine: the underlying driver.Conn is
// not safe for concurrent use, and the arg slice and prepared statement are
// reused across InsertBatch calls.
type Log struct {
	conn driver.Conn         // for Close and Prepare
	args []driver.NamedValue // pooled bind args; ordinals set once, values rewritten per flush
	// TODO: rename to batchSizeStmt
	stmt      driver.Stmt // multi-row INSERT for batchSize entries
	batchSize int         // configured batch size; only batches of this size use stmt
}

// Verify interface implementation
var _ db.DbLog = (*Log)(nil)

// NewLog wraps a connection returned by NewConn. It builds and prepares the
// multi-row INSERT for batchSize entries, so InsertBatch does no per-flush
// string building or prepare; the statement is never re-built or re-prepared.
func NewLog(conn driver.Conn, batchSize int) (*Log, error) {
	if conn == nil {
		return nil, fmt.Errorf("conn cannot be nil")
	}
	if batchSize < 1 {
		return nil, fmt.Errorf("batch size must be at least 1, got %d", batchSize)
	}

	// NewLog wraps connections returned by NewConn, so the conn is modernc's
	// and the assertions below cannot fail.
	pc := conn.(driver.ConnPrepareContext)
	stmt, err := pc.PrepareContext(context.Background(), buildInsertStmt(batchSize))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare batch insert statement: %w", err)
	}

	return &Log{conn: conn, stmt: stmt, batchSize: batchSize}, nil
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
// connection, leaving the prepared statement untouched.
func (l *Log) InsertBatch(batch []db.Log) error {
	if l.conn == nil {
		return ErrConnectionClosed
	}
	if len(batch) == 0 {
		return nil
	}

	l.bindArgs(batch)

	if len(batch) != l.batchSize {
		// Partial flush (e.g. ticker or shutdown drain): execute once through
		// the connection, which prepares and closes the statement internally.
		// The prepared statement is never touched.
		exec := l.conn.(driver.ExecerContext)
		_, err := exec.ExecContext(context.Background(), buildInsertStmt(len(batch)), l.args)
		if err != nil {
			return fmt.Errorf("failed to execute partial insert statement: %w", err)
		}
		return nil
	}

	// The statement always comes from a modernc conn, whose *stmt
	// implements driver.StmtExecContext; the assertion cannot fail.
	exec := l.stmt.(driver.StmtExecContext)
	_, err := exec.ExecContext(context.Background(), l.args)
	if err != nil {
		return fmt.Errorf("failed to execute batch insert statement: %w", err)
	}
	return nil
}

// bindArgs fills l.args with the 4 bind arguments of each entry. The backing
// array is pooled across flushes and only grown when needed; Ordinals are
// position-based and set once, per flush only the Values are rewritten. This
// is safe because the driver copies every value into C memory during bind,
// so nothing retains the args after ExecContext returns.
func (l *Log) bindArgs(batch []db.Log) {
	if cap(l.args) < len(batch)*4 {
		l.args = make([]driver.NamedValue, len(batch)*4)
		for i := range l.args {
			l.args[i].Ordinal = i + 1
		}
	}
	l.args = l.args[:len(batch)*4]
	for i, e := range batch {
		base := i * 4
		l.args[base].Value = e.Level
		l.args[base+1].Value = e.Message
		l.args[base+2].Value = e.JsonData
		l.args[base+3].Value = e.Created
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

// buildInsertStmt returns the multi-row INSERT for batchSize value tuples:
//
//	INSERT INTO logs (level, message, data, created) VALUES (?,?,?,?), (?,?,?,?), ...
//
// Positional '?' placeholders keep the stmt preparable with a single
// argument conversion round for the whole batch.
func buildInsertStmt(batchSize int) string {
	var sb strings.Builder
	sb.Grow(len(insertPrefix) + batchSize*(len(insertTuple)+len(insertSep)))
	sb.WriteString(insertPrefix)
	for i := 0; i < batchSize; i++ {
		if i > 0 {
			sb.WriteString(insertSep)
		}
		sb.WriteString(insertTuple)
	}
	return sb.String()
}

// Ping checks if the specified table exists.
func (l *Log) Ping(tableName string) error {
	if l.conn == nil {
		return ErrConnectionClosed
	}
	exec := l.conn.(driver.ExecerContext)
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", tableName)
	if _, err := exec.ExecContext(context.Background(), query, nil); err != nil {
		return fmt.Errorf("failed to ping table %s: %w", tableName, err)
	}
	return nil
}

// Close closes the underlying SQLite connection and the prepared statement.
func (l *Log) Close() error {
	if l.conn == nil {
		return ErrConnectionClosed
	}
	stmtErr := l.stmt.Close()
	connErr := l.conn.Close()
	l.conn = nil // Set to nil after closing
	l.args = nil
	return errors.Join(stmtErr, connErr)
}

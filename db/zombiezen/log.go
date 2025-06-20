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
	// Use URI filename with performance pragmas
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=off", dbPath)

	conn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open logging connection: %w", err)
	}

	// Additional performance tuning that can't be set via DSN
	//err = sqlitex.Execute(conn, "PRAGMA cache_size=-10000;", nil) // 10MB cache
	//if err != nil {
	//	conn.Close()
	//	return nil, fmt.Errorf("failed to set cache_size: %w", err)
	//}

	return &Log{conn: conn}, nil
}

// InsertBatch writes a batch of log entries to the SQLite database.
// It uses an explicit transaction that will be rolled back on any error.
func (l *Log) InsertBatch(batch []db.Log) error {
	if len(batch) == 0 {
		return nil
	}

	// Start immediate transaction for better concurrency control
	err := sqlitex.Execute(l.conn, "BEGIN IMMEDIATE;", nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case we exit early
	defer func() {
		if err != nil {
			_ = sqlitex.Execute(l.conn, "ROLLBACK;", nil)
		}
	}()

	// Prepare insert statement
	stmt, err := l.conn.Prepare("INSERT INTO logs (level, message, data, created) VALUES ($level, $message, $data, $created)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Finalize()

	// Insert each record
	for _, entry := range batch {
		stmt.SetInt64("$level", entry.Level)
		stmt.SetText("$message", entry.Message)
		stmt.SetText("$data", entry.JsonData)
		stmt.SetText("$created", entry.Created)

		if _, err = stmt.Step(); err != nil {
			stmt.Reset()
			return fmt.Errorf("failed to execute statement for record (msg: %q): %w", entry.Message, err)
		}
		stmt.Reset()
	}

	// Commit transaction if all inserts succeeded
	if err = sqlitex.Execute(l.conn, "COMMIT;", nil); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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

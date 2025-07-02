package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Db struct {
	pool *sqlitex.Pool
}

// Verify interface implementations
var (
	_ db.DbAuth   = (*Db)(nil)
	_ db.DbQueue  = (*Db)(nil)
	_ db.DbConfig = (*Db)(nil)
)

// New creates a new Db instance using an existing pool provided by the user.
// Note: The lifecycle of the provided pool (*sqlitex.Pool) is managed externally.
// This Db type does not close the pool.
func New(pool *sqlitex.Pool) (*Db, error) {
	if pool == nil {
		return nil, fmt.Errorf("provided pool cannot be nil")
	}
	return &Db{pool: pool}, nil
}

// NewConn creates a new SQLite connection with performance optimizations.
func NewConn(dbPath string) (*sqlite.Conn, error) {
	// Use URI filename with performance pragmas
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=off", dbPath)

	conn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open logging connection: %w", err)
	}
	return conn, nil
}

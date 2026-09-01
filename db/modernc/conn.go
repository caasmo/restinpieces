package modernc

import (
	"database/sql"
	"errors"
	"fmt"
)

// NewConn opens a new single SQLite connection with the shared performance
// pragmas (see defaultPragmas). The database file must already exist:
// mode=rw in the DSN makes SQLite fail instead of creating the file.
// Caller pragmas may be passed as a map of pragma name to value; a pragma on
// the same key as a default replaces it, and a pragma in a later map replaces
// the same key from an earlier one.
func NewConn(dbPath string, pragmas ...map[string]string) (*sql.DB, error) {
	dsn := buildDSN(dbPath, pragmas...) + "&mode=rw"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection at %s: %w", dbPath, err)
	}

	// Single connection: the pool never grows beyond one.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// sql.Open is lazy; Ping forces the first connection so that a
	// missing file or bad DSN fails here, at construction time.
	if err := sqlDB.Ping(); err != nil {
		closeErr := sqlDB.Close()
		return nil, errors.Join(fmt.Errorf("failed to open connection at %s: %w", dbPath, err), closeErr)
	}
	return sqlDB, nil
}

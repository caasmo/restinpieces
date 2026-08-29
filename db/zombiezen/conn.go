package zombiezen

import (
	"fmt"

	"zombiezen.com/go/sqlite"
)

// OpenConn opens a new single SQLite connection with performance pragmas.
// The database file must already exist; OpenCreate is not used.
func OpenConn(dbPath string) (*sqlite.Conn, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=off", dbPath)
	conn, err := sqlite.OpenConn(dsn, sqlite.OpenReadWrite|sqlite.OpenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection at %s: %w", dbPath, err)
	}
	return conn, nil
}

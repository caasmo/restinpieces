package zombiezen

import (
	"errors"
	"fmt"

	"zombiezen.com/go/sqlite"
)

// OpenConn opens a new single SQLite connection with performance pragmas.
// The database file must already exist; OpenCreate is not used.
func OpenConn(dbPath string) (conn *sqlite.Conn, err error) {
	conn, err = sqlite.OpenConn("file:"+dbPath, sqlite.OpenReadWrite|sqlite.OpenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection at %s: %w", dbPath, err)
	}
	if err = applyPragmas(conn); err != nil {
		closeErr := conn.Close()
		return nil, errors.Join(fmt.Errorf("failed to apply pragmas at %s: %w", dbPath, err), closeErr)
	}
	return conn, nil
}

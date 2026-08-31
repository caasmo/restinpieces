package zombiezen

import (
	"errors"
	"fmt"

	"zombiezen.com/go/sqlite"
)

// NewConn opens a new single SQLite connection with the shared performance
// pragmas (see defaultPragmas). The database file must already exist;
// OpenCreate is not used. Pragmas beyond the defaults may be passed as full
// statements; they run after the defaults and override them on key collision.
func NewConn(dbPath string, pragmas ...string) (conn *sqlite.Conn, err error) {
	conn, err = sqlite.OpenConn("file:"+dbPath, sqlite.OpenReadWrite|sqlite.OpenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection at %s: %w", dbPath, err)
	}
	if err = applyPragmas(conn, buildPragmas(pragmas)); err != nil {
		closeErr := conn.Close()
		return nil, errors.Join(fmt.Errorf("failed to apply pragmas at %s: %w", dbPath, err), closeErr)
	}
	return conn, nil
}

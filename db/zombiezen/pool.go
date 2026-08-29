package zombiezen

import (
	"fmt"
	"runtime"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// connPragmas are the SQLite pragmas applied to every connection after it is
// opened, pool connections and single connections alike.
//
// zombiezen.com/go/sqlite (v1.4.2) does NOT parse DSN pragma parameters:
// a URI like "file:app.db?_journal_mode=WAL&_synchronous=NORMAL" opens the
// connection with default settings and silently ignores every parameter
// except SQLite's native ones (mode, cache, immutable, vfs, ...). The only
// pragma the library executes itself is "PRAGMA journal_mode=wal", and only
// when the OpenWAL flag is passed. All other settings must be executed
// explicitly after the connection is opened — which is what this list does,
// and why every connection constructor in this package runs applyPragmas.
//
// busy_timeout precedes journal_mode so a WAL conversion can wait on locks
// instead of failing immediately.
var connPragmas = []string{
	"PRAGMA busy_timeout = 5000",
	"PRAGMA journal_mode = WAL",
	"PRAGMA synchronous = NORMAL",
	"PRAGMA foreign_keys = off",
}

// applyPragmas executes the shared pragma list on a freshly opened connection.
func applyPragmas(conn *sqlite.Conn) error {
	for _, pragma := range connPragmas {
		if err := sqlitex.Execute(conn, pragma, nil); err != nil {
			return fmt.Errorf("failed to apply %q: %w", pragma, err)
		}
	}
	return nil
}

// NewPool creates a new Zombiezen SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled, busy_timeout set).
// Every connection gets the shared pragma list via PrepareConn.
func NewPool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()

	pool, err := sqlitex.NewPool("file:"+dbPath, sqlitex.PoolOptions{
		PoolSize:    poolSize,
		PrepareConn: applyPragmas,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

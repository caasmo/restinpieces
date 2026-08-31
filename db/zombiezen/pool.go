package zombiezen

import (
	"fmt"
	"runtime"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// defaultPragmas are the SQLite pragmas applied to every connection after it
// is opened, pool connections and single connections alike.
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
var defaultPragmas = []string{
	"PRAGMA busy_timeout = 5000",
	"PRAGMA journal_mode = WAL",
	"PRAGMA synchronous = NORMAL",
	"PRAGMA foreign_keys = off",
}

// applyPragmas executes the given pragma statements on a freshly opened
// connection, in order. Later statements win: an extra pragma appended after
// a default overrides it.
func applyPragmas(conn *sqlite.Conn, pragmas []string) error {
	for _, pragma := range pragmas {
		if err := sqlitex.Execute(conn, pragma, nil); err != nil {
			return fmt.Errorf("failed to apply %q: %w", pragma, err)
		}
	}
	return nil
}

// buildPragmas returns the pragma list for a new connection: defaultPragmas
// followed by the caller's pragmas, appended verbatim. Because SQLite applies
// them in order, a caller pragma on the same key overrides the default.
// Caller pragmas are never parsed or rewritten.
func buildPragmas(pragmas []string) []string {
	list := make([]string, 0, len(defaultPragmas)+len(pragmas))
	list = append(list, defaultPragmas...)
	list = append(list, pragmas...)
	return list
}

// NewPool creates a new Zombiezen SQLite connection pool with reasonable
// defaults compatible with restinpieces (e.g., WAL mode enabled, busy_timeout
// set). Every connection gets the shared pragma list via PrepareConn.
// Pragmas beyond the defaults may be passed as full statements; they run
// after the defaults and override them on key collision. For example, the
// default busy_timeout of 5 seconds can be raised to 10:
//
//	pool, err := NewPool("app.db", "PRAGMA busy_timeout = 10000")
func NewPool(dbPath string, pragmas ...string) (*sqlitex.Pool, error) {
	list := buildPragmas(pragmas)

	pool, err := sqlitex.NewPool("file:"+dbPath, sqlitex.PoolOptions{
		PoolSize: runtime.NumCPU(),
		PrepareConn: func(conn *sqlite.Conn) error {
			return applyPragmas(conn, list)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

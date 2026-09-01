package modernc

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// defaultPragmas are the SQLite pragmas applied to every connection after it
// is opened, pool connections and single connections alike. They are written
// in the modernc DSN form the driver executes on every new connection:
// "_pragma=name(value)". busy_timeout is placed first by the driver's own
// sorting so a WAL conversion can wait on locks instead of failing
// immediately.
var defaultPragmas = map[string]string{
	"busy_timeout": "5000",
	"journal_mode": "WAL",
	"synchronous":  "NORMAL",
	"foreign_keys": "off",
}

// buildDSN converts a database path and pragma overrides into a modernc DSN.
// The defaults become _pragma query parameters; a caller pragma on the same
// key replaces the default, leaving exactly one pragma per key.
//
// The DSN parameter order is irrelevant: the driver collects every _pragma
// parameter and executes them sorted — busy_timeout first, then the rest in
// case-insensitive lexicographic order — so the non-deterministic map
// iteration order below never influences which pragma runs when.
func buildDSN(dbPath string, pragmas ...map[string]string) string {
	values := make(map[string]string, len(defaultPragmas)+len(pragmas))
	for name, value := range defaultPragmas {
		values[name] = value
	}
	for _, p := range pragmas {
		for name, value := range p {
			values[name] = value
		}
	}

	params := make([]string, 0, len(values)+1)
	for name, value := range values {
		params = append(params, "_pragma="+name+"("+value+")")
	}
	return "file:" + dbPath + "?" + strings.Join(params, "&")
}

// NewPool creates a new modernc SQLite connection pool with reasonable
// defaults compatible with restinpieces (e.g., WAL mode enabled, busy_timeout
// set). Every connection gets the shared pragma list via DSN parameters.
// Caller pragmas may be passed as a map of pragma name to value; a pragma on
// the same key as a default replaces it, and a pragma in a later map replaces
// the same key from an earlier one. For example, the default busy_timeout of
// 5 seconds can be raised to 10:
//
//	pool, err := NewPool("app.db", map[string]string{"busy_timeout": "10000"})
//
// The pool keeps one connection per CPU open, mirroring the previous driver.
func NewPool(dbPath string, pragmas ...map[string]string) (*sql.DB, error) {
	dsn := buildDSN(dbPath, pragmas...)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create modernc pool at %s: %w", dbPath, err)
	}

	// One connection per CPU, like the previous driver's fixed-size pool.
	sqlDB.SetMaxOpenConns(runtime.NumCPU())
	sqlDB.SetMaxIdleConns(runtime.NumCPU())

	// sql.Open is lazy; Ping forces the first connection so that a bad
	// path or DSN fails here, at construction time.
	if err := sqlDB.Ping(); err != nil {
		closeErr := sqlDB.Close()
		return nil, errors.Join(fmt.Errorf("failed to connect modernc pool at %s: %w", dbPath, err), closeErr)
	}
	return sqlDB, nil
}

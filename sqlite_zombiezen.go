package restinpieces

// This file provides helper functions to create SQLite connection pools
// compatible with restinpieces using common drivers (Crawshaw and Zombiezen).
// If your application interacts directly with the database alongside restinpieces,
// it's crucial to use a *single shared pool* to prevent database locking issues (SQLITE_BUSY errors).
// These functions offer reasonable default configurations (like enabling WAL mode)
// suitable for use with restinpieces. You can use these functions to create the
// pool and then pass it to both restinpieces (via options like WithDbCrawshaw)
// and your own application's database access layer.

import (
	"fmt"

	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// WithZombiezenPool configures the App to use the Zombiezen SQLite implementation with an existing pool.
// The user is responsible for creating and managing the lifecycle of the provided pool.
func WithZombiezenPool(pool *sqlitex.Pool) Option {
	return func(i *initializer) {
		dbInstance, err := zombiezen.New(pool)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize zombiezen DB with existing pool: %v", err))
		}
		i.app.SetDb(dbInstance)
		i.dbConfig = dbInstance
	}
}

// NewZombiezenPool creates a new Zombiezen SQLite connection pool with
// reasonable defaults compatible with restinpieces (e.g., WAL mode enabled,
// busy_timeout set). Pragmas beyond the defaults may be passed as full
// statements, e.g. "PRAGMA cache_size = 10000"; they run after the defaults
// and override them on key collision.
func NewZombiezenPool(dbPath string, pragmas ...string) (*sqlitex.Pool, error) {
	return zombiezen.NewPool(dbPath, pragmas...)
}

// NewZombiezenConn creates a new single SQLite connection with the shared
// performance pragmas. The database file must already exist; OpenCreate is
// not used. Pragmas beyond the defaults may be passed as full statements;
// they run after the defaults and override them on key collision.
func NewZombiezenConn(dbPath string, pragmas ...string) (*sqlite.Conn, error) {
	return zombiezen.NewConn(dbPath, pragmas...)
}

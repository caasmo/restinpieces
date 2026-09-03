package restinpieces

// This file provides helper functions to create SQLite connection pools
// compatible with restinpieces. If your application interacts directly with
// the database alongside restinpieces, it's crucial to use a *single shared
// pool* to prevent database locking issues (SQLITE_BUSY errors).
// These functions offer reasonable default configurations (like enabling WAL
// mode) suitable for use with restinpieces. You can use these functions to
// create the pool and then pass it to both restinpieces (via options like
// WithModerncPool) and your own application's database access layer. An
// application that runs its own statements through a `databasesql.Db` registers
// them the same way, by calling `RegisterStmt` during startup.

import (
	"database/sql"
	"fmt"

	"github.com/caasmo/restinpieces/db/databasesql"
)

// WithModerncPool configures the App to use the Modernc SQLite implementation with an existing pool.
// The user is responsible for creating and managing the lifecycle of the provided pool.
func WithModerncPool(pool *sql.DB) Option {
	return func(i *initializer) {
		dbInstance, err := databasesql.New(pool)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize modernc DB with existing pool: %v", err))
		}

		for name, sql := range databasesql.UsersStmts {
			if _, err := dbInstance.RegisterStmt(name, sql); err != nil {
				panic(fmt.Sprintf("failed to register statement: %v", err))
			}
		}

		for name, sql := range databasesql.QueueStmts {
			if _, err := dbInstance.RegisterStmt(name, sql); err != nil {
				panic(fmt.Sprintf("failed to register statement: %v", err))
			}
		}

		i.app.SetDb(dbInstance)
		i.dbConfig = dbInstance
	}
}

// NewModerncPool creates a new Modernc SQLite connection pool with
// reasonable defaults compatible with restinpieces (e.g., WAL mode enabled,
// busy_timeout set). Caller pragmas may be passed as a map of pragma name to
// value, e.g. map[string]string{"cache_size": "10000"}; a pragma on the same
// key as a default replaces it.
func NewModerncPool(dbPath string, pragmas ...map[string]string) (*sql.DB, error) {
	return databasesql.NewPool(dbPath, pragmas...)
}

// NewModerncConn creates a new single SQLite connection with the shared
// performance pragmas. The database file must already exist. Caller pragmas
// may be passed as a map of pragma name to value; a pragma on the same key
// as a default replaces it.
func NewModerncConn(dbPath string, pragmas ...map[string]string) (*sql.DB, error) {
	return databasesql.NewConn(dbPath, pragmas...)
}

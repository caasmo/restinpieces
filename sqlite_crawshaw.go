//go:build sqlite_crawshaw

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
	"runtime"

	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db/crawshaw"
	"github.com/caasmo/restinpieces/core"
)

// WithDbCrawshaw configures the App to use the Crawshaw SQLite implementation with an existing pool.
func WithDbCrawshaw(pool *sqlitex.Pool) core.Option {
	dbInstance, err := crawshaw.New(pool) // Use the renamed New function
	if err != nil {
		// Panic is reasonable here as it indicates a fundamental setup error.
		panic(fmt.Sprintf("failed to initialize crawshaw DB with existing pool: %v", err))
	}
	// Use the renamed app database option
	return core.WithDbApp(dbInstance)
}


// NewCrawshawPool creates a new Crawshaw SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled).
// Use this if your application needs to share the pool with restinpieces.
func NewCrawshawPool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", dbPath)

	// sqlitex.Open with flags=0 defaults to:
	// SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_WAL |
	// SQLITE_OPEN_URI | SQLITE_OPEN_NOMUTEX
	pool, err := sqlitex.Open(initString, 0, poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create default crawshaw pool at %s: %w", dbPath, err)
	}
	return pool, nil
}


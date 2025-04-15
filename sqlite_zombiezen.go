//go:build !sqlite_crawshaw && !sqlite_mattn

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
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/db/zombiezen"
)

// WithDbZombiezen configures the App to use the Zombiezen SQLite implementation with an existing pool.
func WithDbZombiezen(pool *sqlitex.Pool) core.Option {
	dbInstance, err := zombiezen.New(pool) // Use the renamed New function
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zombiezen DB with existing pool: %v", err))
	}
	// Use the renamed app database option
	return core.WithDbApp(dbInstance)
}

// NewZombiezenPool creates a new Zombiezen SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled, busy_timeout set).
// Use this if your application needs to share the pool with restinpieces.
func NewZombiezenPool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()
	// Re-add busy_timeout pragma as part of reasonable defaults for Zombiezen.
	//initString := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)
	initString := fmt.Sprintf("file:%s", dbPath)

	// zombiezen/sqlitex.NewPool with default options uses flags:
	// sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenWAL | sqlite.OpenURI
	pool, err := sqlitex.NewPool(initString, sqlitex.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

var explicitBusyTimeout = 5 * time.Second

// NewZombiezenPerformancePool creates a new Zombiezen SQLite connection pool optimized
// for performance using explicit PRAGMA settings via the DSN string.
func NewZombiezenPerformancePool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()

	// Construct the DSN string with performance PRAGMAs
	// Use DSN parameters: _journal_mode, _synchronous, _busy_timeout, _foreign_keys, _cache_size
	// busy_timeout in DSN is in milliseconds.
	// Set foreign_keys=on for better data integrity.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=%d&_foreign_keys=on&_cache_size=-4000",
		dbPath,
		explicitBusyTimeout.Milliseconds(), // Use milliseconds for _busy_timeout DSN parameter
	)

	// Default OpenFlags (ReadWrite | Create | WAL | URI) are used by NewPool.
	// The URI flag is necessary for the DSN parameters to be parsed.
	pool, err := sqlitex.NewPool(dsn, sqlitex.PoolOptions{
		PoolSize: poolSize,
		// No PrepareConn needed as PRAGMAs are in DSN
	})
	if err != nil {
		// Include the DSN in the error message for easier debugging
		return nil, fmt.Errorf("failed to create performance zombiezen pool at %s using DSN '%s': %w", dbPath, dsn, err)
	}
	return pool, nil
}

// --- Create the pool with the explicit prepare function (Removed as PrepareConn caused issues) ---

// Example DSN string format used above:
// dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=%d&_foreign_keys=on&_cache_size=-4000",
//     dbPath,
//     explicitBusyTimeout.Milliseconds(),
// )

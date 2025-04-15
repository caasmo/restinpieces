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

func prepareConnPerformance(conn *sqlite.Conn) error {
	script := fmt.Sprintf(`
	PRAGMA journal_mode = WAL;
	PRAGMA synchronous = NORMAL;
	PRAGMA busy_timeout = %d;
	PRAGMA foreign_keys = OFF;
	--PRAGMA cache_size = -4000; -- Set cache to 4MB
	`, explicitBusyTimeout.Milliseconds()) // busy_timeout pragma uses milliseconds

	err := sqlitex.ExecuteScript(conn, script, nil)
	if err != nil {
		return fmt.Errorf("failed to apply performance PRAGMAs: %w", err)
	}
	return nil
}

// NewZombiezenPerformancePool creates a new Zombiezen SQLite connection pool optimized
// for performance using explicit PRAGMA settings via ConnPrepareFunc.
func NewZombiezenPerformancePool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()
	// Use the base file path, PRAGMAs are set in prepareConnPerformance
	initString := fmt.Sprintf("file:%s", dbPath)

	// Default OpenFlags (ReadWrite | Create | WAL | URI) are generally fine
	pool, err := sqlitex.NewPool(initString, sqlitex.PoolOptions{
		PoolSize:        poolSize,
		PrepareConn: prepareConnPerformance,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create performance zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

// --- Create the pool with the explicit prepare function ---
// pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
//	PoolSize:        10, // Example pool size
//	ConnPrepareFunc: prepareConnExplicit,
//})

// dsn := fmt.Sprintf("file:%s?_journal=WAL&_synchronous=NORMAL&_timeout=%d&_foreign_keys=on&_cache_size=-4000",
//        dbPath,
//                explicitBusyTimeout.Milliseconds(), // Timeout parameter expects milliseconds

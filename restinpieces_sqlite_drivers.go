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

	crawshawPool "crawshaw.io/sqlite/sqlitex"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex"
)

// NewCrawshawPool creates a new Crawshaw SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled).
// Use this if your application needs to share the pool with restinpieces.
func NewCrawshawPool(dbPath string) (*crawshawPool.Pool, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", dbPath)

	// sqlitex.Open with flags=0 defaults to:
	// SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_WAL |
	// SQLITE_OPEN_URI | SQLITE_OPEN_NOMUTEX
	pool, err := crawshawPool.Open(initString, 0, poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create default crawshaw pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

// NewZombiezenPool creates a new Zombiezen SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled, busy_timeout set).
// Use this if your application needs to share the pool with restinpieces.
func NewZombiezenPool(dbPath string) (*zombiezenPool.Pool, error) {
	poolSize := runtime.NumCPU()
	// Re-add busy_timeout pragma as part of reasonable defaults for Zombiezen.
	//initString := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)
	initString := fmt.Sprintf("file:%s", dbPath)

	// zombiezen/sqlitex.NewPool with default options uses flags:
	// sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenWAL | sqlite.OpenURI
	pool, err := zombiezenPool.NewPool(initString, zombiezenPool.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

var explicitBusyTimeout = 5 * time.Second // Or whatever value you prefer

//func prepareConnExplicit (conn *sqlite.Conn) error {
//	//log.Printf("Preparing connection %p explicitly...", conn)
//
//	// --- Define the explicit PRAGMA settings ---
//
//	// Note 1: OpenFlags like OpenWAL, OpenCreate, OpenURI are applied *before*
//	// this function runs, when sqlitex.Pool calls sqlite.OpenConn internally.
//	// We replicate the *result* of those flags here where possible via PRAGMA.
//
//	// Note 2: Disabling double-quoted strings is done via sqlite3_db_config
//	// by default in OpenConn and cannot be set via PRAGMA. It's already set.
//
//	script := fmt.Sprintf(`
//	-- Replicate effect of default OpenWAL flag
//	PRAGMA journal_mode = WAL;
//
//	-- Set synchronous mode explicitly (NORMAL is common/default for WAL)
//	PRAGMA synchronous = NORMAL;
//
//	-- Set busy timeout explicitly (overrides library's default SetBlockOnBusy handler)
//	PRAGMA busy_timeout = %d;
//
//	-- Explicitly set foreign key constraint handling (SQLite default is OFF)
//	PRAGMA foreign_keys = OFF;
//	-- For most applications, you likely *want* foreign keys ON:
//	-- PRAGMA foreign_keys = ON;
//
//	-- Add any other PRAGMAs you want explicitly set for every connection
//	-- e.g., PRAGMA cache_size = -4000; -- Set cache to 4MB
//	`, explicitBusyTimeout.Milliseconds()) // busy_timeout pragma uses milliseconds
//
//	err := sqlitex.ExecuteScript(conn, script, nil)
//	if err != nil {
//		return fmt.Errorf("failed to apply explicit PRAGMAs: %w", err)
//	}
//
//	//log.Printf("Explicit PRAGMAs applied successfully for connection %p", conn)
//	return nil // Indicate success
//}

// --- Create the pool with the explicit prepare function ---
//pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
//	PoolSize:        10, // Example pool size
//	ConnPrepareFunc: prepareConnExplicit,
//})

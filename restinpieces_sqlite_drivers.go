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

	crawshawPool "crawshaw.io/sqlite/sqlitex"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex"
)

// NewCrawshawPool creates a new Crawshaw SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled).
// Use this if your application needs to share the pool with restinpieces.
func NewCrawshawPool(dbPath string) (*crawshawPool.Pool, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", dbPath)

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

	pool, err := zombiezenPool.NewPool(initString, zombiezenPool.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

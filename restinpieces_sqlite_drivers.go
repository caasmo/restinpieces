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
	"log/slog"
	"runtime"

	crawshawPool "crawshaw.io/sqlite/sqlitex"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex"
)

// NewDefaultCrawshawPool creates a new Crawshaw SQLite connection pool with default settings.
// It uses the number of CPU cores for the pool size and enables WAL mode by default.
func NewDefaultCrawshawPool(dbPath string) (*crawshawPool.Pool, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", dbPath)

	pool, err := crawshawPool.Open(initString, 0, poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create default crawshaw pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

// NewDefaultZombiezenPool creates a new Zombiezen SQLite connection pool with default settings.
// It uses the number of CPU cores for the pool size, enables WAL mode by default, and sets a busy timeout.
func NewDefaultZombiezenPool(dbPath string) (*zombiezenPool.Pool, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", dbPath)

	pool, err := zombiezenPool.NewPool(initString, zombiezenPool.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default zombiezen pool at %s: %w", dbPath, err)
	}
	return pool, nil
}

package restinpieces

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

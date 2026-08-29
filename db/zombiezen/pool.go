package zombiezen

import (
	"fmt"
	"runtime"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"
)

var explicitBusyTimeout = 5 * time.Second

// NewPool creates a new Zombiezen SQLite connection pool with reasonable defaults
// compatible with restinpieces (e.g., WAL mode enabled, busy_timeout set).
func NewPool(dbPath string) (*sqlitex.Pool, error) {
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

// NewPerformancePool creates a new Zombiezen SQLite connection pool optimized
// for performance using explicit PRAGMA settings via the DSN string.
func NewPerformancePool(dbPath string) (*sqlitex.Pool, error) {
	poolSize := runtime.NumCPU()

	// Construct the DSN string with performance PRAGMAs
	// Use DSN parameters: _journal_mode, _synchronous, _busy_timeout, _foreign_keys, _cache_size
	// busy_timeout in DSN is in milliseconds.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=%d&_foreign_keys=off",
		dbPath,
		explicitBusyTimeout.Milliseconds(), // Use milliseconds for _busy_timeout DSN parameter
	)

	// Default OpenFlags (ReadWrite | Create | WAL | URI) are used by NewPool.
	// The URI flag is necessary for the DSN parameters to be parsed.
	pool, err := sqlitex.NewPool(dsn, sqlitex.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create performance zombiezen pool at %s using DSN '%s': %w", dbPath, dsn, err)
	}
	return pool, nil
}

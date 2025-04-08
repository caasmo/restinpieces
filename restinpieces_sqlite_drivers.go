package restinpieces

import (
	"fmt"
	"log/slog"
	"runtime"

	crawshawPool "crawshaw.io/sqlite/sqlitex"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex"
)

// NewDefaultCrawshawPool creates a new Crawshaw SQLite connection pool with default settings.
// It uses the number of CPU cores for the pool size and enables WAL mode.
func NewDefaultCrawshawPool(dbPath string) (*crawshawPool.Pool, error) {
	poolSize := runtime.NumCPU()
	// WAL mode is generally recommended for better concurrency.
	// Use file: URI format for flags.
	// See: https://www.sqlite.org/uri.html
	// See: https://www.sqlite.org/wal.html
	// Note: Litestream requires WAL mode.
	// The flags=0 argument to sqlitex.Open defaults to enabling WAL mode,
	// so no explicit pragma is needed in the URI string for that.
	// It also defaults to SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_URI | SQLITE_OPEN_NOMUTEX.
	initString := fmt.Sprintf("file:%s", dbPath) // Use file: URI to allow flags like WAL

	// flags = 0 enables WAL by default.
	pool, err := crawshawPool.Open(initString, 0, poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create default crawshaw pool at %s: %w", dbPath, err)
	}
	slog.Debug("Default Crawshaw pool created successfully", "path", dbPath, "size", poolSize)
	return pool, nil
}

// NewDefaultZombiezenPool creates a new Zombiezen SQLite connection pool with default settings.
// It uses the number of CPU cores for the pool size, enables WAL mode, and sets a busy timeout.
func NewDefaultZombiezenPool(dbPath string) (*zombiezenPool.Pool, error) {
	poolSize := runtime.NumCPU()
	// zombiezen/go/sqlite/sqlitex.NewPool also defaults to WAL mode when no flags are specified.
	// We still need the file: URI prefix and the busy_timeout pragma.
	initString := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)

	// No explicit OpenFlags needed here; defaults include WAL.
	pool, err := zombiezenPool.NewPool(initString, zombiezenPool.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default zombiezen pool at %s: %w", dbPath, err)
	}
	slog.Debug("Default Zombiezen pool created successfully", "path", dbPath, "size", poolSize)
	return pool, nil
}

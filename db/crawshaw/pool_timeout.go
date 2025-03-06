package crawshaw

import (
	"context"
	"crawshaw.io/sqlite"
	"time"
)

const defaultTimeout = 1 * time.Second

// getWithTimeout attempts to acquire a connection from the pool with a timeout.
// Returns the connection or error if context deadline is exceeded.
func (db *Db) getWithTimeout(ctx context.Context) *sqlite.Conn {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, _ = context.WithTimeout(ctx, defaultTimeout) // Ignore the cancel func
	}

	return db.pool.Get(ctx)
}

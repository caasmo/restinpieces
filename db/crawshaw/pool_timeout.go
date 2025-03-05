package crawshaw

import (
	"context"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"time"
)

// TakeWithTimeout attempts to acquire a connection from the pool with a timeout.
// Returns the connection or error if context deadline is exceeded.
func (p *sqlitex.Pool) TakeWithTimeout(timeout time.Duration) (*sqlite.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	conn := p.Get(ctx)
	if conn == nil {
		return nil, context.DeadlineExceeded
	}
	return conn, nil
}

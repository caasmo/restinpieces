package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"runtime"

	"github.com/caasmo/restinpieces/db"
)

type Db struct {
	pool         *sqlitex.Pool
        //rwConn *sqlitex.Conn
        rwCh         chan *sqlite.Conn
    }

    // Verify interface implementation (non-allocating check)
    var _ db.Db = (*Db)(nil)

// New creates a new Db instance using an existing pool provided by the user.
func New(pool *sqlitex.Pool) (*Db, error) {
	if pool == nil {
		return nil, fmt.Errorf("provided pool cannot be nil")
	}
	// Setup the single writer connection channel (if needed, otherwise remove)
	conn := pool.Get(nil)
	if conn == nil {
		// Don't close the pool here as we don't own it
		return nil, fmt.Errorf("failed to get initial connection from provided pool")
	}
	ch := make(chan *sqlite.Conn, 1)
	go func(initialConn *sqlite.Conn, pool *sqlitex.Pool, ch chan<- *sqlite.Conn) {
		ch <- initialConn
	}(conn, pool, ch) // Pass pool if needed by the goroutine's logic

	return &Db{pool: pool, rwCh: ch}, nil
}


// Close releases resources used by Db. It does NOT close the underlying pool,
// as the pool's lifecycle is managed externally by the user.
func (d *Db) Close() {
	// Handle the writer channel first (ensure connection is returned)
	if d.rwCh != nil {
		select {
		case conn := <-d.rwCh:
			if conn != nil && d.pool != nil {
				d.pool.Put(conn) // Ensure the writer conn is returned
			}
		default:
			// Channel was empty or already closed
		}
		// Consider closing the channel if the writer goroutine expects it
		// close(d.rwCh)
	// Do not close the pool here. The user who created the pool is responsible for closing it.
	// Set pool to nil to prevent further use after Close
	d.pool = nil
	d.rwCh = nil
}

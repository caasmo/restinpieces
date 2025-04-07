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
        ownsPool     bool // Flag to indicate if this instance created the pool
        //rwConn *sqlitex.Conn
        rwCh         chan *sqlite.Conn
    }

    // Verify interface implementation (non-allocating check)
    var _ db.Db = (*Db)(nil)

    // New creates a new Db instance, including creating its own pool.
    func New(path string) (*Db, error) {
        poolSize := runtime.NumCPU()
        // Enable WAL mode and set a busy timeout for better concurrency handling.
        initString := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)

        p, err := sqlitex.Open(initString, 0, poolSize)
        if err != nil {
            return nil, fmt.Errorf("failed to open sqlite pool at %s: %w", path, err)
        }

        // Setup the single writer connection channel (if needed, otherwise remove)
        conn := p.Get(nil)
        if conn == nil {
            p.Close()
            return nil, fmt.Errorf("failed to get initial connection from pool")
        }
        ch := make(chan *sqlite.Conn, 1)
        go func(initialConn *sqlite.Conn, pool *sqlitex.Pool, ch chan<- *sqlite.Conn) {
            ch <- initialConn
        }(conn, p, ch)

        return &Db{pool: p, ownsPool: true, rwCh: ch}, nil
}

// NewWithPool creates a new Db instance using an existing pool.
func NewWithPool(pool *sqlitex.Pool) (*Db, error) {
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

	// ownsPool is false because the pool was provided externally
	return &Db{pool: pool, ownsPool: false, rwCh: ch}, nil
}


// Close releases resources used by Db. It only closes the pool if it owns it.
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
	}

	// Only close the pool if this Db instance created it.
	if d.ownsPool && d.pool != nil {
		err := d.pool.Close()
		if err != nil {
			// Log the error appropriately
			fmt.Printf("Error closing owned sqlite pool: %v\n", err)
		}
	}
	// Set pool to nil to prevent further use after Close
	d.pool = nil
	d.rwCh = nil
}

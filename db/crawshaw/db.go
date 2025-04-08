package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

type Db struct {
	pool *sqlitex.Pool
}

// Verify interface implementations
var _ db.DbAuth = (*Db)(nil)
var _ db.DbQueue = (*Db)(nil)
var _ db.DbLifecycle = (*Db)(nil)

// New creates a new Db instance using an existing pool provided by the user.
func New(pool *sqlitex.Pool) (*Db, error) {
	if pool == nil {
		return nil, fmt.Errorf("provided pool cannot be nil")
	}
	// The pool is managed externally, just store it.
	return &Db{pool: pool}, nil
}

// Close releases resources used by Db. It does NOT close the underlying pool,
// as the pool's lifecycle is managed externally by the user.
// This implementation currently only prevents further use by setting the pool to nil.
func (d *Db) Close() {
	// Do not close the pool here. The user who created the pool is responsible for closing it.
	// Set pool to nil to prevent further use after Close.
	d.pool = nil
}

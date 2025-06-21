package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Db struct {
	pool *sqlitex.Pool
}

// Verify interface implementations
var (
	_ db.DbAuth   = (*Db)(nil)
	_ db.DbQueue  = (*Db)(nil) 
	_ db.DbConfig = (*Db)(nil)
	_ db.DbApp    = (*Db)(nil)
)

// New creates a new Db instance using an existing pool provided by the user.
// Note: The lifecycle of the provided pool (*sqlitex.Pool) is managed externally.
// This Db type does not close the pool.
func New(pool *sqlitex.Pool) (*Db, error) {
	if pool == nil {
		return nil, fmt.Errorf("provided pool cannot be nil")
	}
	// The pool is managed externally, just store it.
	return &Db{pool: pool}, nil
}


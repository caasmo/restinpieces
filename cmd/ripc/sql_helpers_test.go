package main

import (
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newAppDbFromPool builds an appDb from an existing pool. Test-only helper.
func newAppDbFromPool(pool *sqlitex.Pool) *appDb {
	zdb, _ := dbz.New(pool)
	return &appDb{pool: pool, Db: zdb}
}

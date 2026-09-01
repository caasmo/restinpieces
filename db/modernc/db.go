package modernc

import (
	"database/sql"
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

type Db struct {
	db *sql.DB
}

// Verify interface implementations
var (
	_ db.DbAuth   = (*Db)(nil)
	_ db.DbQueue  = (*Db)(nil)
	_ db.DbConfig = (*Db)(nil)
)

// New creates a new Db instance using an existing database provided by the
// user. Note: The lifecycle of the provided *sql.DB is managed externally.
// This Db type does not close the database.
func New(sqlDB *sql.DB) (*Db, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("provided database cannot be nil")
	}
	return &Db{db: sqlDB}, nil
}

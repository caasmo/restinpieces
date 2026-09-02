package databasesql

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newTestUserDB creates a new in-memory SQLite database with the users
// schema and registers the users read statements on it.
func newTestUserDB(t *testing.T) *Db {
	db := newTestDb(t, "app/users.sql")
	registerUsersStmts(t, db)
	return db
}

func TestUsersSuite(t *testing.T) {
	dbtest.UsersSuite{Db: newTestUserDB(t)}.RunAll(t)
}

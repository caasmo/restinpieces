package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newTestUserDB creates a new in-memory SQLite database and applies the users schema.
func newTestUserDB(t *testing.T) *Db {
	return newTestDb(t, "app/users.sql")
}

func TestUsersSuite(t *testing.T) {
	dbtest.UsersSuite{Db: newTestUserDB(t)}.RunAll(t)
}

package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newTestDB creates a new in-memory SQLite database and applies all schemas.
func newTestDB(t *testing.T) *Db {
	return newTestDb(t, "app/app_config.sql")
}

func TestConfigSuite(t *testing.T) {
	dbtest.ConfigSuite{Db: newTestDB(t)}.RunAll(t)
}

package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newTestQueueDB creates a new in-memory SQLite database and applies the job_queue schema.
func newTestQueueDB(t *testing.T) *Db {
	return newTestDb(t, "app/job_queue.sql")
}

func TestQueueSuite(t *testing.T) {
	dbtest.QueueSuite{Db: newTestQueueDB(t)}.RunAll(t)
}

func TestQueueAdminSuite(t *testing.T) {
	dbtest.QueueAdminSuite{Db: newTestQueueDB(t)}.RunAll(t)
}

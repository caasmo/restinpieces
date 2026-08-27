package zombiezen

import (
	"context"
	"io/fs"
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestQueueDB creates a new in-memory SQLite database and applies the job_queue schema.
func newTestQueueDB(t *testing.T) *Db {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}

	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get db connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	sqlBytes, err := fs.ReadFile(schemaFS, "app/job_queue.sql")
	if err != nil {
		t.Fatalf("Failed to read app/job_queue.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app/job_queue.sql: %v", err)
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

func TestQueueSuite(t *testing.T) {
	dbtest.QueueSuite{Db: newTestQueueDB(t)}.RunAll(t)
}

func TestQueueAdminSuite(t *testing.T) {
	dbtest.QueueAdminSuite{Db: newTestQueueDB(t)}.RunAll(t)
}

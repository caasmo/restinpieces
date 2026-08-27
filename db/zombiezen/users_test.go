package zombiezen

import (
	"context"
	"io/fs"
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestUserDB creates a new in-memory SQLite database and applies the users schema.
func newTestUserDB(t *testing.T) *Db {
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
	sqlBytes, err := fs.ReadFile(schemaFS, "app/users.sql")
	if err != nil {
		t.Fatalf("Failed to read app/users.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app/users.sql: %v", err)
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

func TestUsersSuite(t *testing.T) {
	dbtest.UsersSuite{Db: newTestUserDB(t)}.RunAll(t)
}

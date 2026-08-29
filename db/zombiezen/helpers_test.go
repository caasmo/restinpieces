package zombiezen

import (
	"context"
	"io/fs"
	"testing"

	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite/sqlitex"
)

func newTestDb(t *testing.T, schemaPaths ...string) *Db {
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

	schemaFS := sql.FS()
	for _, p := range schemaPaths {
		sqlBytes, err := fs.ReadFile(schemaFS, p)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", p, err)
		}

		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			t.Fatalf("Failed to execute %s: %v", p, err)
		}
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

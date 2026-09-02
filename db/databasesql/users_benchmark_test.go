package databasesql

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newUsersBenchDb creates a benchmark database with the users schema and
// registers the users read statements on it.
func newUsersBenchDb(b *testing.B) *Db {
	db := newBenchDb(b, "app/users.sql")
	registerUsersStmts(b, db)
	return db
}

// BenchmarkUser_GetById_Serial runs the shared GetUserById workload against a
// production-like database file. See dbtest.BenchUser_GetById_Serial.
func BenchmarkUser_GetById_Serial(b *testing.B) {
	dbtest.BenchUser_GetById_Serial(b, newUsersBenchDb(b))
}

// BenchmarkUser_GetById_Parallel runs the shared GetUserById workload under
// contention. See dbtest.BenchUser_GetById_Parallel.
func BenchmarkUser_GetById_Parallel(b *testing.B) {
	dbtest.BenchUser_GetById_Parallel(b, newUsersBenchDb(b))
}

// BenchmarkUser_GetByEmail_Serial runs the shared GetUserByEmail workload
// against a production-like database file. See
// dbtest.BenchUser_GetByEmail_Serial.
func BenchmarkUser_GetByEmail_Serial(b *testing.B) {
	dbtest.BenchUser_GetByEmail_Serial(b, newUsersBenchDb(b))
}

// BenchmarkUser_GetByEmail_Parallel runs the shared GetUserByEmail workload
// under contention. See dbtest.BenchUser_GetByEmail_Parallel.
func BenchmarkUser_GetByEmail_Parallel(b *testing.B) {
	dbtest.BenchUser_GetByEmail_Parallel(b, newUsersBenchDb(b))
}

package databasesql

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkUser_GetById_Serial runs the shared GetUserById workload against a
// production-like database file. See dbtest.BenchUser_GetById_Serial.
func BenchmarkUser_GetById_Serial(b *testing.B) {
	dbtest.BenchUser_GetById_Serial(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkUser_GetById_Parallel runs the shared GetUserById workload under
// contention. See dbtest.BenchUser_GetById_Parallel.
func BenchmarkUser_GetById_Parallel(b *testing.B) {
	dbtest.BenchUser_GetById_Parallel(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkUser_GetByEmail_Serial runs the shared GetUserByEmail workload
// against a production-like database file. See
// dbtest.BenchUser_GetByEmail_Serial.
func BenchmarkUser_GetByEmail_Serial(b *testing.B) {
	dbtest.BenchUser_GetByEmail_Serial(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkUser_GetByEmail_Parallel runs the shared GetUserByEmail workload
// under contention. See dbtest.BenchUser_GetByEmail_Parallel.
func BenchmarkUser_GetByEmail_Parallel(b *testing.B) {
	dbtest.BenchUser_GetByEmail_Parallel(b, newBenchDb(b, "app/users.sql"))
}

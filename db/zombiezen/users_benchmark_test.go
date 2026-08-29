package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkGetUserById_Serial runs the shared GetUserById workload against a
// production-like database file. See dbtest.BenchGetUserById_Serial.
func BenchmarkGetUserById_Serial(b *testing.B) {
	dbtest.BenchGetUserById_Serial(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkGetUserById_Parallel runs the shared GetUserById workload under
// contention. See dbtest.BenchGetUserById_Parallel.
func BenchmarkGetUserById_Parallel(b *testing.B) {
	dbtest.BenchGetUserById_Parallel(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkGetUserByEmail_Serial runs the shared GetUserByEmail workload
// against a production-like database file. See
// dbtest.BenchGetUserByEmail_Serial.
func BenchmarkGetUserByEmail_Serial(b *testing.B) {
	dbtest.BenchGetUserByEmail_Serial(b, newBenchDb(b, "app/users.sql"))
}

// BenchmarkGetUserByEmail_Parallel runs the shared GetUserByEmail workload
// under contention. See dbtest.BenchGetUserByEmail_Parallel.
func BenchmarkGetUserByEmail_Parallel(b *testing.B) {
	dbtest.BenchGetUserByEmail_Parallel(b, newBenchDb(b, "app/users.sql"))
}

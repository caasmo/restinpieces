package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkInsertJob_Serial runs the shared InsertJob workload against a
// production-like database file. See dbtest.BenchInsertJob_Serial.
func BenchmarkInsertJob_Serial(b *testing.B) {
	dbtest.BenchInsertJob_Serial(b, newBenchDb(b, "app/job_queue.sql"))
}

// BenchmarkInsertJob_Parallel runs the shared InsertJob workload under
// contention. See dbtest.BenchInsertJob_Parallel.
func BenchmarkInsertJob_Parallel(b *testing.B) {
	dbtest.BenchInsertJob_Parallel(b, newBenchDb(b, "app/job_queue.sql"))
}

// BenchmarkClaim_Serial runs the shared Claim workload against a
// production-like database file. See dbtest.BenchClaim_Serial.
func BenchmarkClaim_Serial(b *testing.B) {
	dbtest.BenchClaim_Serial(b, newBenchDb(b, "app/job_queue.sql"))
}

// BenchmarkClaim_Parallel runs the shared Claim workload under contention.
// See dbtest.BenchClaim_Parallel.
func BenchmarkClaim_Parallel(b *testing.B) {
	dbtest.BenchClaim_Parallel(b, newBenchDb(b, "app/job_queue.sql"))
}

// BenchmarkMarkCompleted_Serial runs the shared MarkCompleted workload
// against a production-like database file. See
// dbtest.BenchMarkCompleted_Serial.
func BenchmarkMarkCompleted_Serial(b *testing.B) {
	dbtest.BenchMarkCompleted_Serial(b, newBenchDb(b, "app/job_queue.sql"))
}

// BenchmarkMarkCompleted_Parallel runs the shared MarkCompleted workload
// under contention. See dbtest.BenchMarkCompleted_Parallel.
func BenchmarkMarkCompleted_Parallel(b *testing.B) {
	dbtest.BenchMarkCompleted_Parallel(b, newBenchDb(b, "app/job_queue.sql"))
}

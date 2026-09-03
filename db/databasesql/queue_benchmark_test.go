package databasesql

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// newQueueBenchDb creates a benchmark database with the job_queue schema and
// registers the queue statements on it.
func newQueueBenchDb(b *testing.B) *Db {
	db := newBenchDb(b, "app/job_queue.sql")
	registerQueueStmts(b, db)
	return db
}

// BenchmarkQueue_InsertJob_Serial runs the shared InsertJob workload against a
// production-like database file. See dbtest.BenchQueue_InsertJob_Serial.
func BenchmarkQueue_InsertJob_Serial(b *testing.B) {
	dbtest.BenchQueue_InsertJob_Serial(b, newQueueBenchDb(b))
}

// BenchmarkQueue_InsertJob_Parallel runs the shared InsertJob workload under
// contention. See dbtest.BenchQueue_InsertJob_Parallel.
func BenchmarkQueue_InsertJob_Parallel(b *testing.B) {
	dbtest.BenchQueue_InsertJob_Parallel(b, newQueueBenchDb(b))
}

// BenchmarkQueue_Claim_Serial runs the shared Claim workload against a
// production-like database file. See dbtest.BenchQueue_Claim_Serial.
func BenchmarkQueue_Claim_Serial(b *testing.B) {
	dbtest.BenchQueue_Claim_Serial(b, newQueueBenchDb(b))
}

// BenchmarkQueue_Claim_Parallel runs the shared Claim workload under contention.
// See dbtest.BenchQueue_Claim_Parallel.
func BenchmarkQueue_Claim_Parallel(b *testing.B) {
	dbtest.BenchQueue_Claim_Parallel(b, newQueueBenchDb(b))
}

// BenchmarkQueue_MarkCompleted_Serial runs the shared MarkCompleted workload
// against a production-like database file. See
// dbtest.BenchQueue_MarkCompleted_Serial.
func BenchmarkQueue_MarkCompleted_Serial(b *testing.B) {
	dbtest.BenchQueue_MarkCompleted_Serial(b, newQueueBenchDb(b))
}

// BenchmarkQueue_MarkCompleted_Parallel runs the shared MarkCompleted workload
// under contention. See dbtest.BenchQueue_MarkCompleted_Parallel.
func BenchmarkQueue_MarkCompleted_Parallel(b *testing.B) {
	dbtest.BenchQueue_MarkCompleted_Parallel(b, newQueueBenchDb(b))
}

// Benchmarks for DbLog.
//
// Measured: InsertBatch — writes a batch of logs, the only per-request path.
// Not measured:
//   Ping  — checks the connection is alive, run once at startup.
//   Close — closes the connection, run once at shutdown.

package dbtest

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
)

// BenchLog_InsertBatch measures one InsertBatch call against the provided
// log database, one call at a time. Each call inserts n entries in
// one transaction, which is how the framework writes logs.
func BenchLog_InsertBatch(b *testing.B, logDB db.DbLog, n int) {
	batch := make([]db.Log, n)
	for i := 0; i < n; i++ {
		batch[i] = db.Log{
			Level:    1,
			Message:  "bench log message",
			JsonData: `{"key":"value"}`,
			Created:  db.TimeFormat(time.Now()),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		err := logDB.InsertBatch(batch)
		if err != nil {
			b.Fatalf("InsertBatch failed: %v", err)
		}
	}
	// Report per-message throughput so N10/N50/N100 are directly comparable.
	// sec/op is per InsertBatch call (N logs), msg/sec is per log.
	if b.N > 0 {
		nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		b.ReportMetric(float64(n)/(nsPerOp/1e9), "msg/sec")
	}
}

package dbtest

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
)

const logBatchSize = 10

// BenchInsertBatch_Serial measures one InsertBatch call against the provided
// log database, one call at a time. Each call inserts logBatchSize entries in
// one transaction, which is how the framework writes logs. The log database
// uses a single connection, so there is no pool to contend over and only a
// serial variant exists.
func BenchInsertBatch_Serial(b *testing.B, logDB db.DbLog) {
	batch := make([]db.Log, logBatchSize)
	for i := 0; i < logBatchSize; i++ {
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
}

package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkInsertBatch_Serial runs the shared InsertBatch workload against a
// production-like log database. See dbtest.BenchInsertBatch_Serial.
func BenchmarkInsertBatch_Serial(b *testing.B) {
	dbtest.BenchInsertBatch_Serial(b, newBenchLog(b))
}

package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkLog_InsertBatch_Serial runs the shared InsertBatch workload against a
// production-like log database. See dbtest.BenchLog_InsertBatch_Serial.
func BenchmarkLog_InsertBatch_Serial(b *testing.B) {
	dbtest.BenchLog_InsertBatch_Serial(b, newBenchLog(b))
}

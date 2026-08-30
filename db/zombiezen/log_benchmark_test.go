package zombiezen

import (
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkLog_InsertBatch runs the shared InsertBatch workload against a
// production-like log database for varying batch sizes. See dbtest.BenchLog_InsertBatch.
func BenchmarkLog_InsertBatch(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			dbtest.BenchLog_InsertBatch(b, newBenchLog(b), n)
		})
	}
}

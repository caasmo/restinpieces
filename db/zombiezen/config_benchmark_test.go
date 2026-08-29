package zombiezen

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkGetConfig_Serial runs the shared GetConfig workload against a
// production-like database file. See dbtest.BenchGetConfig_Serial.
func BenchmarkGetConfig_Serial(b *testing.B) {
	dbtest.BenchGetConfig_Serial(b, newBenchDb(b, "app/app_config.sql"))
}

// BenchmarkInsertConfig_Serial runs the shared InsertConfig workload against
// a production-like database file. See dbtest.BenchInsertConfig_Serial.
func BenchmarkInsertConfig_Serial(b *testing.B) {
	dbtest.BenchInsertConfig_Serial(b, newBenchDb(b, "app/app_config.sql"))
}

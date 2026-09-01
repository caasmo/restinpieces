package databasesql

import (
	"testing"

	"github.com/caasmo/restinpieces/db/dbtest"
)

// BenchmarkConfig_Get_Serial runs the shared GetConfig workload against a
// production-like database file. See dbtest.BenchConfig_Get_Serial.
func BenchmarkConfig_Get_Serial(b *testing.B) {
	dbtest.BenchConfig_Get_Serial(b, newBenchDb(b, "app/app_config.sql"))
}

// BenchmarkConfig_Insert_Serial runs the shared InsertConfig workload against
// a production-like database file. See dbtest.BenchConfig_Insert_Serial.
func BenchmarkConfig_Insert_Serial(b *testing.B) {
	dbtest.BenchConfig_Insert_Serial(b, newBenchDb(b, "app/app_config.sql"))
}

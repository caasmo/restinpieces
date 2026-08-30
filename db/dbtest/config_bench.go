package dbtest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

const configCount = 10

// benchConfigContent is the config blob used by the config benchmarks. Config
// blobs are the biggest payloads in the database, so a 2KB blob is
// representative.
var benchConfigContent = []byte(strings.Repeat("a", 2048))

// seedBenchConfigs creates configCount configs, one per scope, and returns
// the scope names. Seeding happens before b.ResetTimer(), so it is never
// measured.
func seedBenchConfigs(b *testing.B, benchDB db.DbConfig, configCount int) []string {
	b.Helper()

	scopes := make([]string, 0, configCount)
	for i := 0; i < configCount; i++ {
		scope := fmt.Sprintf("bench-%d", i)
		err := benchDB.InsertConfig(scope, benchConfigContent, "toml", "bench")
		if err != nil {
			b.Fatalf("failed to seed config: %v", err)
		}
		scopes = append(scopes, scope)
	}
	return scopes
}

// BenchConfig_Get_Serial measures one GetConfig call against the provided
// database, one call at a time. The scopes rotate so the reads hit different
// rows like real traffic. Generation 0 is the latest config for a scope,
// which is what the config reload path reads.
func BenchConfig_Get_Serial(b *testing.B, benchDB db.DbConfig) {
	scopes := seedBenchConfigs(b, benchDB, configCount)

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		_, _, err := benchDB.GetConfig(scopes[i%len(scopes)], 0)
		if err != nil {
			b.Fatalf("GetConfig failed: %v", err)
		}
		i++
	}
}

// BenchConfig_Insert_Serial measures one InsertConfig call against the
// provided database, one call at a time. The scopes rotate so the inserts
// spread across rows like real traffic. The same content blob is reused: the
// insert cost does not depend on the blob contents.
func BenchConfig_Insert_Serial(b *testing.B, benchDB db.DbConfig) {
	scopes := make([]string, configCount)
	for i := 0; i < configCount; i++ {
		scopes[i] = fmt.Sprintf("bench-%d", i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		err := benchDB.InsertConfig(scopes[i%len(scopes)], benchConfigContent, "toml", "bench")
		if err != nil {
			b.Fatalf("InsertConfig failed: %v", err)
		}
		i++
	}
}

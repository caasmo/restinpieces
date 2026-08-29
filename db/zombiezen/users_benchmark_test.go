package zombiezen

import (
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

const userCount = 100

// seedBenchUsers creates userCount users in the benchmark database and
// returns their IDs. Seeding happens before b.ResetTimer(), so it is never
// measured.
func seedBenchUsers(b *testing.B, benchDB *Db, userCount int) []string {
	b.Helper()

	ids := make([]string, 0, userCount)
	for i := 0; i < userCount; i++ {
		user, err := benchDB.CreateUserWithPassword(db.User{
			Email:    fmt.Sprintf("bench-%d@example.com", i),
			Password: "bench-password",
		})
		if err != nil {
			b.Fatalf("failed to seed user: %v", err)
		}
		ids = append(ids, user.ID)
	}
	return ids
}

// BenchmarkGetUserById_Serial measures one GetUserById call against a real
// database file, one call at a time. This is the floor cost of the database
// lookup that every authenticated request performs.
//
// Reference result (8-core i7-8550U): ~11µs/op, ~90K lookups/sec (values
// vary with machine load). This is the latency of a single lookup when the
// server is idle.
func BenchmarkGetUserById_Serial(b *testing.B) {
	benchDB := newBenchDb(b, "app/users.sql")
	ids := seedBenchUsers(b, benchDB, userCount)

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		_, err := benchDB.GetUserById(ids[i%len(ids)])
		if err != nil {
			b.Fatalf("GetUserById failed: %v", err)
		}
		i++
	}
}

// BenchmarkGetUserById_Parallel measures GetUserById under contention: one
// goroutine per CPU, all sharing the same database file through the pool.
// This exposes pool and WAL lock contention that serial benches hide.
//
// Reference result (8-core i7-8550U): ~2.8µs/op reported (values vary with
// machine load). The number looks far lower than the serial one, but it is
// aggregate: total wall time divided by all iterations across the
// goroutines. Each goroutine actually waits ~22µs per lookup under
// contention, while the pool still delivers ~360K lookups/sec total. The
// two benchmarks are far apart because the parallel one reports throughput,
// not per-op latency.
func BenchmarkGetUserById_Parallel(b *testing.B) {
	benchDB := newBenchDb(b, "app/users.sql")
	ids := seedBenchUsers(b, benchDB, userCount)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := benchDB.GetUserById(ids[i%len(ids)])
			if err != nil {
				b.Errorf("GetUserById failed: %v", err)
			}
			i++
		}
	})
}

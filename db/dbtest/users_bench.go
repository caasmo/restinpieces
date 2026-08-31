// Benchmarks for DbAuth.
//
// Measured: GetUserById, GetUserByEmail — run on every login and authenticated request.
// Not measured:
//   CreateUserWithPassword, CreateUserWithOauth2 — run once per user at signup.
//   UpdatePassword, UpdateEmail, UpdateVerified  — rare profile changes, not per-request.

package dbtest

import (
	"fmt"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

const userCount = 100

// seedBenchUsers creates userCount users in the benchmark database and
// returns their IDs and emails. Seeding happens before b.ResetTimer(), so it
// is never measured.
func seedBenchUsers(b *testing.B, benchDB db.DbAuth, userCount int) (ids []string, emails []string) {
	b.Helper()

	ids = make([]string, 0, userCount)
	emails = make([]string, 0, userCount)
	for i := 0; i < userCount; i++ {
		email := fmt.Sprintf("bench-%d@example.com", i)
		user, err := benchDB.CreateUserWithPassword(db.User{
			Email:    email,
			Password: "bench-password",
		})
		if err != nil {
			b.Fatalf("failed to seed user: %v", err)
		}
		ids = append(ids, user.ID)
		emails = append(emails, email)
	}
	return ids, emails
}

// BenchUser_GetById_Serial measures one GetUserById call against the provided
// database, one call at a time. This is the floor cost of the database lookup
// that every authenticated request performs.
func BenchUser_GetById_Serial(b *testing.B, benchDB db.DbAuth) {
	ids, _ := seedBenchUsers(b, benchDB, userCount)

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

// BenchUser_GetById_Parallel measures GetUserById under contention: one
// goroutine per CPU, all sharing the same database through their pool. This
// exposes pool and WAL lock contention that serial benches hide.
func BenchUser_GetById_Parallel(b *testing.B, benchDB db.DbAuth) {
	ids, _ := seedBenchUsers(b, benchDB, userCount)

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

// BenchUser_GetByEmail_Serial measures one GetUserByEmail call against the
// provided database, one call at a time. This is the cost of the lookup the
// login flow performs.
func BenchUser_GetByEmail_Serial(b *testing.B, benchDB db.DbAuth) {
	_, emails := seedBenchUsers(b, benchDB, userCount)

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		_, err := benchDB.GetUserByEmail(emails[i%len(emails)])
		if err != nil {
			b.Fatalf("GetUserByEmail failed: %v", err)
		}
		i++
	}
}

// BenchUser_GetByEmail_Parallel measures GetUserByEmail under contention: one
// goroutine per CPU, all sharing the same database through their pool.
func BenchUser_GetByEmail_Parallel(b *testing.B, benchDB db.DbAuth) {
	_, emails := seedBenchUsers(b, benchDB, userCount)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := benchDB.GetUserByEmail(emails[i%len(emails)])
			if err != nil {
				b.Errorf("GetUserByEmail failed: %v", err)
			}
			i++
		}
	})
}

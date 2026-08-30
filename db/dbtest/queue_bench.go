package dbtest

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

const jobCount = 100

// benchQueue is the queue interface the queue benchmarks exercise: the queue
// operations plus the admin operations needed to read job IDs back after
// seeding. Any driver implementing both interfaces can run the same
// workload.
type benchQueue interface {
	db.DbQueue
	db.DbQueueAdmin
}

// seedBenchJobs creates jobCount jobs in the benchmark database and returns
// their IDs. InsertJob returns no ID, so the IDs are read back with ListJobs.
// Seeding happens before b.ResetTimer(), so it is never measured.
func seedBenchJobs(b *testing.B, benchDB benchQueue, jobCount int) []int64 {
	b.Helper()

	for i := 0; i < jobCount; i++ {
		err := benchDB.InsertJob(db.Job{
			JobType: "bench_job",
			Payload: json.RawMessage(fmt.Sprintf(`{"n":%d}`, i)),
		})
		if err != nil {
			b.Fatalf("failed to seed job: %v", err)
		}
	}

	jobs, err := benchDB.ListJobs(0)
	if err != nil {
		b.Fatalf("failed to list seeded jobs: %v", err)
	}
	if len(jobs) != jobCount {
		b.Fatalf("expected %d seeded jobs, got %d", jobCount, len(jobs))
	}

	ids := make([]int64, 0, jobCount)
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	return ids
}

// BenchQueue_InsertJob_Serial measures one InsertJob call against the provided
// database, one call at a time.
func BenchQueue_InsertJob_Serial(b *testing.B, benchDB benchQueue) {
	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		err := benchDB.InsertJob(db.Job{
			JobType: "bench_job",
			Payload: json.RawMessage(fmt.Sprintf(`{"n":%d}`, i)),
		})
		if err != nil {
			b.Fatalf("InsertJob failed: %v", err)
		}
		i++
	}
}

// BenchQueue_InsertJob_Parallel measures InsertJob under contention: one goroutine
// per CPU, all inserting into the same database. This exposes the writer lock
// contention that serial benches hide.
func BenchQueue_InsertJob_Parallel(b *testing.B, benchDB benchQueue) {
	b.ReportAllocs()
	b.ResetTimer()

	var nextPayload atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := nextPayload.Add(1) - 1
			err := benchDB.InsertJob(db.Job{
				JobType: "bench_job",
				Payload: json.RawMessage(fmt.Sprintf(`{"n":%d}`, n)),
			})
			if err != nil {
				b.Errorf("InsertJob failed: %v", err)
			}
		}
	})
}

// BenchQueue_Claim_Serial measures one Claim(1) call against the provided database,
// one call at a time. Claim drains the queue, so each iteration inserts a
// fresh job before claiming it; the insert runs with the timer stopped.
func BenchQueue_Claim_Serial(b *testing.B, benchDB benchQueue) {
	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		b.StopTimer()
		err := benchDB.InsertJob(db.Job{
			JobType: "bench_job",
			Payload: json.RawMessage(fmt.Sprintf(`{"n":%d}`, i)),
		})
		if err != nil {
			b.Fatalf("failed to insert job for claim: %v", err)
		}
		b.StartTimer()

		jobs, err := benchDB.Claim(1)
		if err != nil {
			b.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 1 {
			b.Fatalf("expected 1 job, got %d", len(jobs))
		}
		i++
	}
}

// BenchQueue_Claim_Parallel measures Claim(1) under contention: one goroutine per
// CPU, each inserting a fresh job and claiming it. Claim drains the queue, so
// the insert runs with the timer stopped.
func BenchQueue_Claim_Parallel(b *testing.B, benchDB benchQueue) {
	b.ReportAllocs()
	b.ResetTimer()

	var nextPayload atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := nextPayload.Add(1) - 1

			b.StopTimer()
			err := benchDB.InsertJob(db.Job{
				JobType: "bench_job",
				Payload: json.RawMessage(fmt.Sprintf(`{"n":%d}`, n)),
			})
			if err != nil {
				b.Errorf("failed to insert job for claim: %v", err)
			}
			b.StartTimer()

			jobs, err := benchDB.Claim(1)
			if err != nil {
				b.Errorf("Claim failed: %v", err)
			}
			if len(jobs) != 1 {
				b.Errorf("expected 1 job, got %d", len(jobs))
			}
		}
	})
}

// BenchQueue_MarkCompleted_Serial measures one MarkCompleted call against the
// provided database, one call at a time. The IDs rotate across the seeded
// jobs so the updates hit different rows like real traffic.
func BenchQueue_MarkCompleted_Serial(b *testing.B, benchDB benchQueue) {
	ids := seedBenchJobs(b, benchDB, jobCount)

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		err := benchDB.MarkCompleted(ids[i%len(ids)])
		if err != nil {
			b.Fatalf("MarkCompleted failed: %v", err)
		}
		i++
	}
}

// BenchQueue_MarkCompleted_Parallel measures MarkCompleted under contention: one
// goroutine per CPU, all updating the seeded jobs through the database. This
// exposes the writer lock contention that serial benches hide.
func BenchQueue_MarkCompleted_Parallel(b *testing.B, benchDB benchQueue) {
	ids := seedBenchJobs(b, benchDB, jobCount)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			err := benchDB.MarkCompleted(ids[i%len(ids)])
			if err != nil {
				b.Errorf("MarkCompleted failed: %v", err)
			}
			i++
		}
	})
}

package zombiezen

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestQueueDB creates a new in-memory SQLite database and applies the job_queue schema.
func newTestQueueDB(t *testing.T) *Db {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}

	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get db connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	sqlBytes, err := fs.ReadFile(schemaFS, "app/job_queue.sql")
	if err != nil {
		t.Fatalf("Failed to read app/job_queue.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app/job_queue.sql: %v", err)
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

func TestJobLifecycle(t *testing.T) {
	testDB := newTestQueueDB(t)
	var claimedJob *db.Job

	t.Run("Insert", func(t *testing.T) {
		err := testDB.InsertJob(db.Job{
			JobType: "test_job",
			Payload: json.RawMessage(`{"key":"value"}`),
		})
		if err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		jobs, err := testDB.ListJobs(0)
		if err != nil {
			t.Fatalf("ListJobs failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job, got %d", len(jobs))
		}
		if jobs[0].Status != "pending" {
			t.Errorf("expected job status to be 'pending', got %q", jobs[0].Status)
		}
	})

	t.Run("Claim", func(t *testing.T) {
		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected to claim 1 job, got %d", len(jobs))
		}
		claimedJob = jobs[0]
		if claimedJob.Status != "processing" {
			t.Errorf("expected claimed job status to be 'processing', got %q", claimedJob.Status)
		}
		if claimedJob.Attempts != 1 {
			t.Errorf("expected attempts to be 1, got %d", claimedJob.Attempts)
		}
		if claimedJob.LockedAt.IsZero() {
			t.Error("expected LockedAt to be set")
		}
	})

	t.Run("ClaimEmpty", func(t *testing.T) {
		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim (empty) failed: %v", err)
		}
		if len(jobs) != 0 {
			t.Fatalf("expected to claim 0 jobs, got %d", len(jobs))
		}
	})

	t.Run("MarkCompleted", func(t *testing.T) {
		err := testDB.MarkCompleted(claimedJob.ID)
		if err != nil {
			t.Fatalf("MarkCompleted failed: %v", err)
		}
		jobs, _ := testDB.ListJobs(0)
		if jobs[0].Status != "completed" {
			t.Errorf("expected job status to be 'completed', got %q", jobs[0].Status)
		}
		if jobs[0].CompletedAt.IsZero() {
			t.Error("expected CompletedAt to be set")
		}
	})

	t.Run("MarkFailed", func(t *testing.T) {
		// Insert a new job to fail
		err := testDB.InsertJob(db.Job{JobType: "fail_job"})
		if err != nil {
			t.Fatalf("InsertJob for failure test failed: %v", err)
		}
		jobs, err := testDB.Claim(1)
		if err != nil || len(jobs) != 1 {
			t.Fatalf("Claim for failure test failed, claimed %d jobs", len(jobs))
		}

		failJob := jobs[0]
		errMsg := "it failed"
		err = testDB.MarkFailed(failJob.ID, errMsg)
		if err != nil {
			t.Fatalf("MarkFailed failed: %v", err)
		}

		allJobs, _ := testDB.ListJobs(0)
		var foundJob *db.Job
		for _, j := range allJobs {
			if j.ID == failJob.ID {
				foundJob = j
				break
			}
		}

		if foundJob == nil {
			t.Fatal("failed to find the failed job")
		}
		if foundJob.Status != "failed" {
			t.Errorf("expected status to be 'failed', got %q", foundJob.Status)
		}
		if foundJob.LastError != errMsg {
			t.Errorf("expected last error to be %q, got %q", errMsg, foundJob.LastError)
		}
	})
}

func TestRecurrentJob(t *testing.T) {
	testDB := newTestQueueDB(t)

	// 1. Insert a recurrent job
	recurrentJob := db.Job{
		JobType:   "recurrent_job",
		Recurrent: true,
		Interval:  1 * time.Hour,
	}
	err := testDB.InsertJob(recurrentJob)
	if err != nil {
		t.Fatalf("InsertJob for recurrent job failed: %v", err)
	}

	// 2. Claim the job
	jobs, err := testDB.Claim(1)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("Claim for recurrent job failed, claimed %d jobs", len(jobs))
	}
	claimedJob := jobs[0]

	// 3. Define the next job instance, mimicking the scheduler's behavior
	// by creating a new, unique payload for the next run.
	nextScheduledFor := time.Now().Add(claimedJob.Interval)
	recurrentPayload := map[string]string{"scheduled_for": nextScheduledFor.Format(time.RFC3339)}
	payloadJSON, _ := json.Marshal(recurrentPayload)

	nextJob := db.Job{
		JobType:      claimedJob.JobType,
		Payload:      payloadJSON, // Use the new, unique payload
		PayloadExtra: claimedJob.PayloadExtra,
		MaxAttempts:  claimedJob.MaxAttempts,
		Recurrent:    claimedJob.Recurrent,
		Interval:     claimedJob.Interval,
		ScheduledFor: nextScheduledFor,
	}

	// 4. Mark the current job as completed, which should re-queue it
	err = testDB.MarkRecurrentCompleted(claimedJob.ID, nextJob)
	if err != nil {
		t.Fatalf("MarkRecurrentCompleted failed: %v", err)
	}

	// 5. Verification
	allJobs, _ := testDB.ListJobs(0)
	if len(allJobs) != 2 {
		t.Fatalf("expected 2 jobs after recurrent completion, got %d", len(allJobs))
	}

	var completedJob, newPendingJob *db.Job
	for _, j := range allJobs {
		if j.ID == claimedJob.ID {
			completedJob = j
		} else {
			newPendingJob = j
		}
	}

	if completedJob == nil || newPendingJob == nil {
		t.Fatal("did not find both completed and new pending jobs")
	}

	if completedJob.Status != "completed" {
		t.Errorf("expected original job status to be 'completed', got %q", completedJob.Status)
	}
	if newPendingJob.Status != "pending" {
		t.Errorf("expected new job status to be 'pending', got %q", newPendingJob.Status)
	}
	if newPendingJob.ScheduledFor.IsZero() {
		t.Error("expected new job to have a future ScheduledFor time")
	}
}

func TestJobAdminAndEdgeCases(t *testing.T) {
	t.Run("ListJobs", func(t *testing.T) {
		testDB := newTestQueueDB(t)
		if err := testDB.InsertJob(db.Job{JobType: "job1"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		if err := testDB.InsertJob(db.Job{JobType: "job2"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		jobs, err := testDB.ListJobs(0)
		if err != nil {
			t.Fatalf("ListJobs(0) failed: %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("expected 2 jobs, got %d", len(jobs))
		}

		jobs, err = testDB.ListJobs(1)
		if err != nil {
			t.Fatalf("ListJobs(1) failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Errorf("expected 1 job with limit, got %d", len(jobs))
		}
	})

	t.Run("DeleteJob", func(t *testing.T) {
		testDB := newTestQueueDB(t)
		if err := testDB.InsertJob(db.Job{JobType: "to_delete"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		jobs, _ := testDB.ListJobs(0)
		jobID := jobs[0].ID

		err := testDB.DeleteJob(jobID)
		if err != nil {
			t.Fatalf("DeleteJob failed: %v", err)
		}

		jobs, _ = testDB.ListJobs(0)
		for _, j := range jobs {
			if j.ID == jobID {
				t.Fatal("job was not deleted")
			}
		}
	})

	t.Run("DeleteNonExistentJob", func(t *testing.T) {
		testDB := newTestQueueDB(t)
		err := testDB.DeleteJob(99999)
		if err == nil {
			t.Error("expected an error when deleting a non-existent job, but got nil")
		}
	})

	t.Run("ScheduledJobNotClaimed", func(t *testing.T) {
		testDB := newTestQueueDB(t)
		err := testDB.InsertJob(db.Job{
			JobType:      "future_job",
			ScheduledFor: time.Now().Add(1 * time.Hour),
		})
		if err != nil {
			t.Fatalf("InsertJob for future job failed: %v", err)
		}

		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim for future job failed: %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 jobs to be claimed, got %d", len(jobs))
		}
	})
}

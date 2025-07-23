package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	//"sync"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"github.com/caasmo/restinpieces/queue/executor"
	"zombiezen.com/go/sqlite/sqlitex"
)

// --- Test Helpers ---

// FuncHandler is an adapter to allow the use of ordinary functions as JobHandlers.
type FuncHandler func(ctx context.Context, job db.Job) error

// Handle calls f(ctx, job).
func (f FuncHandler) Handle(ctx context.Context, job db.Job) error {
	return f(ctx, job)
}

// newTestQueueDB creates a new in-memory SQLite database for testing.
func newTestQueueDB(t *testing.T) *zombiezen.Db {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{PoolSize: 1})
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

	db, err := zombiezen.New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

// newTestScheduler creates a scheduler with its real dependencies for testing.
func newTestScheduler(t *testing.T, cfg config.Scheduler) (*Scheduler, *zombiezen.Db) {
	t.Helper()

	testDB := newTestQueueDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	exec := executor.NewExecutor(nil)

	fullCfg := &config.Config{Scheduler: cfg}
	provider := config.NewProvider(fullCfg)

	scheduler := NewScheduler(provider, testDB, exec, logger)

	return scheduler, testDB
}

// --- Test Cases ---

func TestScheduler_Lifecycle(t *testing.T) {
	cfg := config.Scheduler{
		Interval: config.Duration{Duration: 10 * time.Millisecond},
	}
	scheduler, _ := newTestScheduler(t, cfg)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Scheduler.Start() failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := scheduler.Stop(ctx); err != nil {
		t.Fatalf("Scheduler.Stop() failed: %v", err)
	}
}

func TestScheduler_ProcessJobs(t *testing.T) {
	cfg := config.Scheduler{
		Interval:              config.Duration{Duration: 100 * time.Millisecond},
		MaxJobsPerTick:        10,
		ConcurrencyMultiplier: 2,
	}

	t.Run("Success - Non-recurrent", func(t *testing.T) {
		scheduler, testDB := newTestScheduler(t, cfg)

		var executedJobType string
		handler := FuncHandler(func(ctx context.Context, job db.Job) error {
			executedJobType = job.JobType
			return nil
		})
		scheduler.Executor().Register("test_success", handler)

		if err := testDB.InsertJob(db.Job{JobType: "test_success"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		scheduler.processJobs()

		if executedJobType != "test_success" {
			t.Errorf("expected job 'test_success' to be executed, got %q", executedJobType)
		}

		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 jobs to be claimable, got %d", len(jobs))
		}
	})

	t.Run("Success - Recurrent", func(t *testing.T) {
		scheduler, testDB := newTestScheduler(t, cfg)
		scheduler.Executor().Register("recurrent_job", FuncHandler(func(ctx context.Context, job db.Job) error {
			return nil
		}))

		recurrentJob := db.Job{JobType: "recurrent_job", Recurrent: true, Interval: 1 * time.Hour}
		if err := testDB.InsertJob(recurrentJob); err != nil {
			t.Fatalf("InsertJob for recurrent job failed: %v", err)
		}

		scheduler.processJobs()

		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job to be claimable, got %d", len(jobs))
		}
		if jobs[0].JobType != "recurrent_job" {
			t.Errorf("claimed wrong job type: got %q, want %q", jobs[0].JobType, "recurrent_job")
		}
	})

	t.Run("Failure - Execution Error", func(t *testing.T) {
		scheduler, testDB := newTestScheduler(t, cfg)
		expectedErr := errors.New("executor failed")
		scheduler.Executor().Register("test_failure", FuncHandler(func(ctx context.Context, job db.Job) error {
			return expectedErr
		}))

		if err := testDB.InsertJob(db.Job{JobType: "test_failure"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		scheduler.processJobs()

		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job to be claimable, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status != "processing" {
			t.Errorf("expected job status to be 'processing', got %q", job.Status)
		}
		if job.LastError != expectedErr.Error() {
			t.Errorf("unexpected error message: got %q, want %q", job.LastError, expectedErr.Error())
		}
	})

	t.Run("Failure - Timeout", func(t *testing.T) {
		scheduler, testDB := newTestScheduler(t, cfg)
		scheduler.Executor().Register("test_timeout", FuncHandler(func(ctx context.Context, job db.Job) error {
			return context.DeadlineExceeded
		}))

		if err := testDB.InsertJob(db.Job{JobType: "test_timeout"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		scheduler.processJobs()

		jobs, err := testDB.Claim(1)
		if err != nil {
			t.Fatalf("Claim failed: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job to be claimable, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status != "processing" {
			t.Errorf("expected job status to be 'processing', got %q", job.Status)
		}
		if job.LastError != "job execution timed out" {
			t.Errorf("unexpected error message: got %q, want %q", job.LastError, "job execution timed out")
		}
	})
}

// TODO
//func aTestScheduler_ShutdownCancellation(t *testing.T) {
//	cfg := config.Scheduler{Interval: config.Duration{Duration: 100 * time.Millisecond}, MaxJobsPerTick: 1}
//	scheduler, testDB := newTestScheduler(t, cfg)
//
//	jobStarted := make(chan struct{})
//	var wg sync.WaitGroup
//	wg.Add(1)
//
//	scheduler.Executor().Register("cancellable", FuncHandler(func(ctx context.Context, job db.Job) error {
//		close(jobStarted)
//		<-ctx.Done() // Wait for cancellation
//		wg.Done()
//		return ctx.Err()
//	}))
//
//	if err := testDB.InsertJob(db.Job{JobType: "cancellable"}); err != nil {
//		t.Fatalf("InsertJob failed: %v", err)
//	}
//
//	// Start the scheduler
//	if err := scheduler.Start(); err != nil {
//		t.Fatalf("Start failed: %v", err)
//	}
//
//	// Wait for the job to be picked up by the scheduler's ticker
//	select {
//	case <-jobStarted:
//		// It started, now stop the scheduler
//	case <-time.After(2 * time.Second):
//		t.Fatal("timed out waiting for job to start")
//	}
//
//	// Stop the scheduler, which will cancel the context for processJobs
//	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
//	defer cancel()
//	if err := scheduler.Stop(stopCtx); err != nil {
//		t.Fatalf("Stop failed: %v", err)
//	}
//
//	// Wait for the job handler to complete to ensure DB updates are finished
//	wg.Wait()
//
//	// Check the job's final state
//	jobs, err := testDB.Claim(1)
//	if err != nil {
//		t.Fatalf("Claim failed: %v", err)
//	}
//	if len(jobs) != 1 {
//		t.Fatalf("expected 1 failed job to be claimable, got %d", len(jobs))
//	}
//	if jobs[0].LastError != "job execution canceled" {
//		t.Errorf("expected last error to be 'job execution canceled', got %q", jobs[0].LastError)
//	}
//}

func TestNextRecurrentJob(t *testing.T) {
	now := time.Now()
	interval := 1 * time.Hour
	scheduledFor := now.Add(-interval)

	completedJob := db.Job{
		ID:           1,
		JobType:      "my_recurrent_job",
		PayloadExtra: json.RawMessage(`{"meta":"data"}`),
		MaxAttempts:  5,
		Recurrent:    true,
		Interval:     interval,
		CreatedAt:    now.Add(-2 * interval),
		ScheduledFor: scheduledFor,
	}

	newJob := nextRecurrentJob(completedJob)

	if newJob.JobType != completedJob.JobType {
		t.Errorf("JobType mismatch: got %s, want %s", newJob.JobType, completedJob.JobType)
	}
	if !newJob.Recurrent {
		t.Error("Expected new job to be recurrent")
	}
	if newJob.Interval != completedJob.Interval {
		t.Errorf("Interval mismatch: got %v, want %v", newJob.Interval, completedJob.Interval)
	}
	if newJob.MaxAttempts != completedJob.MaxAttempts {
		t.Errorf("MaxAttempts mismatch: got %d, want %d", newJob.MaxAttempts, completedJob.MaxAttempts)
	}
	if newJob.CreatedAt != completedJob.CreatedAt {
		t.Errorf("CreatedAt should be preserved, got %v, want %v", newJob.CreatedAt, completedJob.CreatedAt)
	}

	expectedScheduledFor := completedJob.ScheduledFor.Add(completedJob.Interval)
	if !newJob.ScheduledFor.Equal(expectedScheduledFor) {
		t.Errorf("ScheduledFor mismatch: got %v, want %v", newJob.ScheduledFor, expectedScheduledFor)
	}

	var payload struct {
		ScheduledFor time.Time `json:"scheduled_for"`
	}
	if err := json.Unmarshal(newJob.Payload, &payload); err != nil {
		t.Fatalf("Failed to unmarshal new job payload: %v", err)
	}
	if !payload.ScheduledFor.Equal(expectedScheduledFor) {
		t.Errorf("Payload ScheduledFor mismatch: got %v, want %v", payload.ScheduledFor, expectedScheduledFor)
	}
}

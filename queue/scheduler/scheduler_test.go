package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/queue/executor"
)

// FuncHandler is an adapter to allow the use of ordinary functions as JobHandlers.
type FuncHandler func(ctx context.Context, job db.Job) error

// Handle calls f(ctx, job).
func (f FuncHandler) Handle(ctx context.Context, job db.Job) error {
	return f(ctx, job)
}

// newTestScheduler creates a scheduler with a stub db.DbQueue for testing.
func newTestScheduler(t *testing.T, cfg config.Scheduler, queue db.DbQueue) *Scheduler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	exec := executor.NewExecutor(nil)

	fullCfg := &config.Config{Scheduler: cfg}
	provider := config.NewProvider(fullCfg)

	return NewScheduler(provider, queue, exec, logger)
}

func TestScheduler_Lifecycle(t *testing.T) {
	cfg := config.Scheduler{
		Interval: config.Duration{Duration: 10 * time.Millisecond},
	}
	scheduler := newTestScheduler(t, cfg, &mock.Db{})

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Scheduler.Start() failed: %v", err)
	}

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
		var markCompletedIDs []int64
		var markFailedCalled bool
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{ID: 1, JobType: "test_success"}}, nil
			},
			MarkCompletedFunc: func(jobID int64) error {
				markCompletedIDs = append(markCompletedIDs, jobID)
				return nil
			},
			MarkFailedFunc: func(jobID int64, errMsg string) error {
				markFailedCalled = true
				return nil
			},
		}
		scheduler := newTestScheduler(t, cfg, queue)

		var executedJobType string
		scheduler.Executor().Register("test_success", FuncHandler(func(ctx context.Context, job db.Job) error {
			executedJobType = job.JobType
			return nil
		}))

		scheduler.processJobs()

		if executedJobType != "test_success" {
			t.Errorf("expected job 'test_success' to be executed, got %q", executedJobType)
		}
		if len(markCompletedIDs) != 1 || markCompletedIDs[0] != 1 {
			t.Errorf("expected MarkCompleted to be called with job 1, got %v", markCompletedIDs)
		}
		if markFailedCalled {
			t.Error("MarkFailed was called for a successful job")
		}
	})

	t.Run("Success - Recurrent", func(t *testing.T) {
		scheduledFor := time.Now().Add(10 * time.Minute)
		var recurrentCompletedID int64
		var recurrentNewJob db.Job
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{
					ID:           1,
					JobType:      "recurrent_job",
					Recurrent:    true,
					Interval:     time.Hour,
					ScheduledFor: scheduledFor,
				}}, nil
			},
			MarkRecurrentCompletedFunc: func(completedJobID int64, newJob db.Job) error {
				recurrentCompletedID = completedJobID
				recurrentNewJob = newJob
				return nil
			},
		}
		scheduler := newTestScheduler(t, cfg, queue)

		scheduler.Executor().Register("recurrent_job", FuncHandler(func(ctx context.Context, job db.Job) error {
			return nil
		}))

		scheduler.processJobs()

		if recurrentCompletedID != 1 {
			t.Errorf("expected MarkRecurrentCompleted to be called with job 1, got %d", recurrentCompletedID)
		}
		if recurrentNewJob.JobType != "recurrent_job" {
			t.Errorf("expected next job type 'recurrent_job', got %q", recurrentNewJob.JobType)
		}
		if !recurrentNewJob.Recurrent {
			t.Error("expected next job to be recurrent")
		}
		if recurrentNewJob.Interval != time.Hour {
			t.Errorf("expected next job interval to be 1h, got %v", recurrentNewJob.Interval)
		}
		expectedScheduledFor := scheduledFor.Add(time.Hour)
		if !recurrentNewJob.ScheduledFor.Equal(expectedScheduledFor) {
			t.Errorf("expected next job scheduled for %v, got %v", expectedScheduledFor, recurrentNewJob.ScheduledFor)
		}
	})

	t.Run("Failure - Execution Error", func(t *testing.T) {
		var failedID int64
		var failedMsg string
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{ID: 1, JobType: "test_failure"}}, nil
			},
			MarkFailedFunc: func(jobID int64, errMsg string) error {
				failedID = jobID
				failedMsg = errMsg
				return nil
			},
		}
		scheduler := newTestScheduler(t, cfg, queue)

		expectedErr := errors.New("executor failed")
		scheduler.Executor().Register("test_failure", FuncHandler(func(ctx context.Context, job db.Job) error {
			return expectedErr
		}))

		scheduler.processJobs()

		if failedID != 1 {
			t.Errorf("expected MarkFailed to be called with job 1, got %d", failedID)
		}
		if failedMsg != expectedErr.Error() {
			t.Errorf("expected error message %q, got %q", expectedErr.Error(), failedMsg)
		}
	})

	t.Run("Failure - Timeout", func(t *testing.T) {
		var failedID int64
		var failedMsg string
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{ID: 1, JobType: "test_timeout"}}, nil
			},
			MarkFailedFunc: func(jobID int64, errMsg string) error {
				failedID = jobID
				failedMsg = errMsg
				return nil
			},
		}
		scheduler := newTestScheduler(t, cfg, queue)

		scheduler.Executor().Register("test_timeout", FuncHandler(func(ctx context.Context, job db.Job) error {
			return context.DeadlineExceeded
		}))

		scheduler.processJobs()

		if failedID != 1 {
			t.Errorf("expected MarkFailed to be called with job 1, got %d", failedID)
		}
		if failedMsg != "job execution timed out" {
			t.Errorf("expected error message %q, got %q", "job execution timed out", failedMsg)
		}
	})
}

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

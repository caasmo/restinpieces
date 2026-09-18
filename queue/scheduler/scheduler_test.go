package scheduler

import (
	"context"
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
		recurrentCfg := cfg
		recurrentCfg.Jobs = config.Jobs{
			"recurrent": {JobType: "recurrent_job", Interval: config.Duration{Duration: time.Hour}, Activated: true},
		}
		var recurrentCompletedID int64
		var recurrentNewJob db.Job
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{
					ID:           1,
					JobType:      "recurrent_job",
					ScheduledFor: scheduledFor,
				}}, nil
			},
			MarkRecurrentCompletedFunc: func(completedJobID int64, newJob db.Job) error {
				recurrentCompletedID = completedJobID
				recurrentNewJob = newJob
				return nil
			},
		}
		scheduler := newTestScheduler(t, recurrentCfg, queue)

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
		expectedScheduledFor := scheduledFor.Add(time.Hour)
		if !recurrentNewJob.ScheduledFor.Equal(expectedScheduledFor) {
			t.Errorf("expected next job scheduled for %v, got %v", expectedScheduledFor, recurrentNewJob.ScheduledFor)
		}
	})

	t.Run("Success - Deactivated entry", func(t *testing.T) {
		deactivatedCfg := cfg
		deactivatedCfg.Jobs = config.Jobs{
			"recurrent": {JobType: "recurrent_job", Interval: config.Duration{Duration: time.Hour}},
		}
		var markCompletedID int64
		var markRecurrentCalled bool
		queue := &mock.Db{
			ClaimFunc: func(limit int) ([]*db.Job, error) {
				return []*db.Job{{ID: 1, JobType: "recurrent_job"}}, nil
			},
			MarkCompletedFunc: func(jobID int64) error {
				markCompletedID = jobID
				return nil
			},
			MarkRecurrentCompletedFunc: func(completedJobID int64, newJob db.Job) error {
				markRecurrentCalled = true
				return nil
			},
		}
		scheduler := newTestScheduler(t, deactivatedCfg, queue)

		scheduler.Executor().Register("recurrent_job", FuncHandler(func(ctx context.Context, job db.Job) error {
			return nil
		}))

		scheduler.processJobs()

		if markCompletedID != 1 {
			t.Errorf("expected MarkCompleted to be called with job 1, got %d", markCompletedID)
		}
		if markRecurrentCalled {
			t.Error("MarkRecurrentCompleted was called for a deactivated entry")
		}
	})

	t.Run("Add activated jobs", func(t *testing.T) {
		jobsCfg := cfg
		jobsCfg.Jobs = config.Jobs{
			"acme_cert": {JobType: "job_type_acme_cert", Interval: config.Duration{Duration: time.Hour}, Activated: true},
			"paused":    {JobType: "job_type_paused", Interval: config.Duration{Duration: time.Hour}},
		}
		var addedJobs []db.Job
		queue := &mock.Db{
			SeedRecurrentFunc: func(jobs []db.Job) error {
				addedJobs = append(addedJobs, jobs...)
				return nil
			},
		}
		scheduler := newTestScheduler(t, jobsCfg, queue)

		before := time.Now()
		scheduler.processJobs()

		if len(addedJobs) != 1 {
			t.Fatalf("expected 1 added job, got %d", len(addedJobs))
		}
		if addedJobs[0].JobType != "job_type_acme_cert" {
			t.Errorf("expected added type 'job_type_acme_cert', got %q", addedJobs[0].JobType)
		}
		earliest := before.Add(time.Hour)
		latest := time.Now().Add(time.Hour)
		if addedJobs[0].ScheduledFor.Before(earliest) || addedJobs[0].ScheduledFor.After(latest) {
			t.Errorf("expected scheduled_for between %v and %v, got %v", earliest, latest, addedJobs[0].ScheduledFor)
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

func TestNextRecurrent(t *testing.T) {
	scheduledFor := time.Now().Add(-time.Hour)
	entry := config.JobEntry{
		JobType:   "job_type_acme_cert",
		Interval:  config.Duration{Duration: 6 * time.Hour},
		Activated: true,
	}
	completedJob := db.Job{
		ID:           1,
		JobType:      entry.JobType,
		ScheduledFor: scheduledFor,
	}

	newJob := nextRecurrent(completedJob, entry)

	if newJob.JobType != entry.JobType {
		t.Errorf("JobType mismatch: got %s, want %s", newJob.JobType, entry.JobType)
	}
	expectedScheduledFor := scheduledFor.Add(entry.Interval.Duration)
	if !newJob.ScheduledFor.Equal(expectedScheduledFor) {
		t.Errorf("ScheduledFor mismatch: got %v, want %v", newJob.ScheduledFor, expectedScheduledFor)
	}
	if len(newJob.Payload) != 0 {
		t.Errorf("expected empty payload, got %s", newJob.Payload)
	}
}

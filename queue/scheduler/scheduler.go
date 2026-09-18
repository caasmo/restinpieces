package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/executor"
	"golang.org/x/sync/errgroup"
)

// TODO remove
const (
	DefaultConcurrencyMultiplier = 2
)

// Scheduler handles scheduled jobs
type Scheduler struct {
	configProvider *config.Provider
	db             db.DbQueue
	executor       executor.JobExecutor

	// logger is used for structured logging
	logger *slog.Logger

	// ctx is the context used to control the scheduler's lifecycle
	// It allows graceful shutdown when Stop() is called from outside.
	// The context is passed to all job execution goroutines.
	ctx context.Context

	// cancel is the CancelFunc associated with ctx
	// It is called in the Stop method to initiate shutdown of the scheduler
	// and all running jobs.
	cancel context.CancelFunc

	// shutdownDone is a channel that will be closed when the scheduler
	// has completely shut down and all jobs have finished.
	// Used to signal completion of the shutdown process.
	shutdownDone chan struct{}
}

func NewScheduler(configProvider *config.Provider, db db.DbQueue, executor executor.JobExecutor, logger *slog.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		configProvider: configProvider,
		db:             db,
		executor:       executor,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		shutdownDone:   make(chan struct{}),
	}
}

// Name returns the name of the daemon for logging/identification.
func (s *Scheduler) Name() string {
	return "Scheduler"
}

// Start begins the job scheduler operation by creating a long running goroutine
// that will create goroutines to handle backend jobs.
// It returns nil immediately as the main work happens in the background goroutine.
func (s *Scheduler) Start() error {
	go func() {
		// Get initial interval from provider
		interval := s.configProvider.Get().Scheduler.Interval.Duration
		s.logger.Info("⏰scheduler: starting", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			// Inside the loop, potentially re-fetch interval if it needs to be dynamic
			// currentInterval := s.configProvider.Get().Scheduler.Interval
			// if currentInterval != interval {
			//    ticker.Reset(currentInterval)
			//    interval = currentInterval
			//    s.logger.Info("⏰scheduler: interval updated", "new_interval", interval)
			// }
			// For now, we use the initial interval.
			select {
			case <-s.ctx.Done():
				s.logger.Info("⏰scheduler: received shutdown signal")
				// Wait for all jobs to complete
				//if err := s.eg.Wait(); err != nil {
				//	s.logger.Error("Error waiting for jobs to complete", "err", err)
				//}
				close(s.shutdownDone) // Signal that scheduler has completely shut down
				return
			case <-ticker.C:
				s.processJobs()
			}
		}
	}()
	return nil // Start returns immediately, background goroutine handles work/errors
}

// Stop signals the scheduler to stop and waits for all jobs to complete
// or the context to be canceled, whichever comes first
func (s *Scheduler) Stop(ctx context.Context) error {
	s.logger.Info("⏰scheduler: stopping")
	s.cancel()

	// Wait for either scheduler completion or context timeout
	select {
	case <-s.shutdownDone:
		s.logger.Info("⏰scheduler: stopped gracefully")
		return nil
	case <-ctx.Done():
		s.logger.Info("⏰scheduler: shutdown timed out")
		return ctx.Err()
	}
}

func (s *Scheduler) processJobs() {
	// Get current scheduler config from provider for this tick
	schedulerCfg := s.configProvider.Get().Scheduler

	addErr := s.seedRecurrent(schedulerCfg.Jobs)
	if addErr != nil {
		s.logger.Error("⏰scheduler: failed to seed recurrent jobs", "err", addErr)
	}

	// Claim jobs up to configured limit per tick
	jobs, err := s.db.Claim(schedulerCfg.MaxJobsPerTick)
	if err != nil {
		s.logger.Error("⏰scheduler: failed to claim jobs", "err", err)
		return
	}

	s.logger.Info("⏰scheduler: tick claimed jobs", "count", len(jobs))

	// Create a new error group for this batch of jobs
	// Use the scheduler's context as parent to ensure jobs receive shutdown signal
	g, ctx := errgroup.WithContext(s.ctx) // <- Shutdown context
	g.SetLimit(runtime.NumCPU() * schedulerCfg.ConcurrencyMultiplier)

	var processed int
	for _, job := range jobs {
		jobCopy := job // Create a copy to avoid closure issues
		g.Go(func() error {
			// Create job-specific timeout context that inherits from the group context
			// TODO timeout to conf
			jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()

			// Execute job with proper timeout while still respecting global cancellation
			err := s.executeJobWithContext(jobCtx, *jobCopy)

			// Handle job completion status
			switch {
			case err == nil:
				s.logger.Info("⏰scheduler: job execution successful", "jobID", jobCopy.ID, "jobType", jobCopy.JobType)

				entry, found := s.configProvider.Get().Scheduler.Jobs.Get(jobCopy.JobType)
				if !found || !entry.Activated {
					if updateErr := s.db.MarkCompleted(jobCopy.ID); updateErr != nil {
						s.logger.Error("⏰scheduler: failed to mark job as completed", "jobID", jobCopy.ID, "error", updateErr)
					} else {
						s.logger.Info("⏰scheduler: job is not active in config, marked completed", "jobID", jobCopy.ID)
					}
					processed++
					break // Exit the switch case for this job
				}

				// Handle recurrent jobs
				newJob := nextRecurrent(*jobCopy, entry)
				if updateErr := s.db.MarkRecurrentCompleted(jobCopy.ID, newJob); updateErr != nil {
					s.logger.Error("⏰scheduler: failed to mark recurrent job completed and schedule next", "jobID", jobCopy.ID, "error", updateErr)
				} else {
					s.logger.Info("⏰scheduler: recurrent job completed and next job scheduled", "completedJobID", jobCopy.ID, "nextScheduledFor", newJob.ScheduledFor.Format(time.RFC3339))
				}
				processed++

			case errors.Is(err, context.DeadlineExceeded):
				msg := "job execution timed out"
				s.logger.Warn("⏰scheduler: job timeout",
					"jobID", jobCopy.ID,
					"jobType", jobCopy.JobType,
					"error", err)
				if updateErr := s.db.MarkFailed(jobCopy.ID, msg); updateErr != nil {
					s.logger.Error("⏰scheduler: failed to mark job as timed out",
						"jobID", jobCopy.ID,
						"error", updateErr)
				}

			case errors.Is(err, context.Canceled):
				msg := "job execution canceled"
				s.logger.Info("⏰scheduler: job canceled",
					"jobID", jobCopy.ID,
					"jobType", jobCopy.JobType,
					"error", err)
				if updateErr := s.db.MarkFailed(jobCopy.ID, msg); updateErr != nil {
					s.logger.Error("⏰scheduler: failed to mark job as interrupted",
						"jobID", jobCopy.ID,
						"error", updateErr)
				}

			default:
				s.logger.Error("⏰scheduler: job execution failed",
					"jobID", jobCopy.ID,
					"jobType", jobCopy.JobType,
					"error", err)
				if updateErr := s.db.MarkFailed(jobCopy.ID, err.Error()); updateErr != nil {
					s.logger.Error("⏰scheduler: failed to mark job as failed",
						"jobID", jobCopy.ID,
						"error", updateErr)
				}
			}

			return err
		})
	}

	// Wait for all jobs in this batch to complete or for the parent context to be canceled
	// returns the first error that was encountered, or nil if none occurred.
	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Info("⏰scheduler: job batch interrupted due to shutdown")
		} else {
			s.logger.Error("⏰scheduler: received one or more tick errors. First:", "err", err)
		}
	}

	if len(jobs) > 0 {
		s.logger.Info("⏰scheduler: finished processing claimed jobs", "success", processed, "total", len(jobs))
	}
}

func (s *Scheduler) executeJobWithContext(ctx context.Context, job db.Job) error {
	// Initial context check
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Log job starting
	s.logger.Info("⏰scheduler: calling executor", "job_id", job.ID, "job_type", job.JobType, "attempt", job.Attempts)

	// Use the executor to handle the job
	return s.executor.Execute(ctx, job)
}

// seedRecurrent adds one run to the queue for every activated job in the
// config that has no run in the queue yet. A job with a pending, processing
// or failed row is skipped: it is already queued, running, or waiting for a
// retry.
//
// A deactivated job gets no new run. A run that is already queued still runs,
// and no new run is added after it.
func (s *Scheduler) seedRecurrent(configJobs config.Jobs) error {
	now := time.Now()
	newJobs := make([]db.Job, 0, len(configJobs))

	for _, entry := range configJobs {
		if !entry.Activated {
			continue
		}
		newJobs = append(newJobs, db.Job{
			JobType:      entry.JobType,
			ScheduledFor: now.Add(entry.Interval.Duration),
		})
	}
	if len(newJobs) == 0 {
		return nil
	}

	return s.db.SeedRecurrent(newJobs)
}

// nextRecurrent builds the next run of a job that just completed. The next
// run is one interval after the completed run's scheduled time. The interval
// is read from the current config, so changing it takes effect from the next
// run on. The new run has no payload: a job declared in config has no
// parameters.
func nextRecurrent(completedJob db.Job, entry config.JobEntry) db.Job {
	return db.Job{
		JobType:      completedJob.JobType,
		ScheduledFor: completedJob.ScheduledFor.Add(entry.Interval.Duration),
	}
}

// Executor returns the job executor used by the scheduler.
// This allows external components (like the server) to register handlers directly.
func (s *Scheduler) Executor() executor.JobExecutor {
	return s.executor
}

// the key is to use context aware packages for db, etc. and periodically check
// (in for loops or multi stage executors) for  <-ctx.Done()

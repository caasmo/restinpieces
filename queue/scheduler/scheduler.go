package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"golang.org/x/sync/errgroup"
)

// TODOremove
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

// Start begins the job scheduler operation by creting a long runnig goroutine
// that will create gorotines to handle backend jobs
func (s *Scheduler) Start() {
	go func() {
		// Get initial interval from provider
		interval := s.configProvider.Get().Scheduler.Interval
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
				s.logger.Info("⏰scheduler: job completed successfully",
					"jobID", jobCopy.ID,
					"jobType", jobCopy.JobType)
				if updateErr := s.db.MarkCompleted(jobCopy.ID); updateErr != nil {
					s.logger.Error("⏰scheduler: failed to mark job as completed",
						"jobID", jobCopy.ID,
						"error", updateErr)
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

func (s *Scheduler) executeJobWithContext(ctx context.Context, job queue.Job) error {
	// Initial context check
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Log job starting
	s.logger.Info("⏰scheduler: calling executor", "job_id", job.ID, "job_type", job.JobType, "attempt", job.Attempts)

	// Use the executor to handle the job
	return s.executor.Execute(ctx, job)
}

// the key is to use context aware packages for db, etc. and periodically check
// (in for loops or multi stage executors) for  <-ctx.Done()

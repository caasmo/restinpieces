package scheduler

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"runtime"
	"time"
	"errors"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// TODOremove
const (
	DefaultConcurrencyMultiplier = 2
)

// Scheduler handles scheduled jobs
type Scheduler struct {
	// cfg contains the scheduler configuration including interval and max jobs per tick
	cfg config.Scheduler

	// db is the database connection used to fetch and update jobs
	db db.Db

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

// NewScheduler creates a new scheduler
func NewScheduler(cfg config.Scheduler, db db.Db) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate concurrency limit based on multiplier and CPU cores
	//concurrency := runtime.NumCPU() * cfg.ConcurrencyMultiplier

	//g, ctx := errgroup.WithContext(ctx)
	//g.SetLimit(concurrency)

	return &Scheduler{
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		db:           db,
		shutdownDone: make(chan struct{}),
	}
}

// Start begins the job scheduler operation by creting a long runnig goroutine
// that will create gorotines to handle backend jobs
func (s *Scheduler) Start() {
	go func() {
		slog.Info("Starting job scheduler", "interval", s.cfg.Interval)
		ticker := time.NewTicker(s.cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				slog.Info("Job scheduler received shutdown signal")
				// Wait for all jobs to complete
				//if err := s.eg.Wait(); err != nil {
				//	slog.Error("Error waiting for jobs to complete", "err", err)
				//}
				close(s.shutdownDone) // Signal that scheduler has completely shut down
				return
			case <-ticker.C:
				slog.Debug("Scheduler tick - processing jobs")
				s.processJobs()
			}
		}
	}()
}

// Stop signals the scheduler to stop and waits for all jobs to complete
// or the context to be canceled, whichever comes first
func (s *Scheduler) Stop(ctx context.Context) error {
	slog.Info("Stopping job scheduler")
	s.cancel()

	// Wait for either scheduler completion or context timeout
	select {
	case <-s.shutdownDone:
		slog.Info("Job scheduler stopped gracefully")
		return nil
	case <-ctx.Done():
		slog.Info("Job scheduler shutdown timed out")
		return ctx.Err()
	}
}

func (s *Scheduler) processJobs() {
    // Claim jobs up to configured limit per tick
    jobs, err := s.db.Claim(s.cfg.MaxJobsPerTick)
    if err != nil {
        slog.Error("Failed to claim jobs", "err", err)
        return
    }

    slog.Info("Claimed jobs", "count", len(jobs))
    
    // Create a new error group for this batch of jobs
    // Use the scheduler's context as parent to ensure jobs receive shutdown signal
    g, ctx := errgroup.WithContext(s.ctx) // <- Shutdown context
    g.SetLimit(runtime.NumCPU() * s.cfg.ConcurrencyMultiplier)
    
    var processed int
    for _, job := range jobs {
        jobCopy := job // Create a copy to avoid closure issues
        g.Go(func() error {
            // Create job-specific timeout context that inherits from the group context
			// TODO timeout to conf
            jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
            defer cancel()
            
            // Execute job with proper timeout while still respecting global cancellation
            err := executeJobWithContext(jobCtx, *jobCopy)
            
            // Handle job completion status
            if err == nil {
                if updateErr := s.db.MarkCompleted(jobCopy.ID); updateErr != nil {
                    slog.Error("Failed to mark job as completed", "jobID", jobCopy.ID, "err", updateErr)
                }
                processed++
            } else if errors.Is(err, context.DeadlineExceeded) {
				msg := "scheduler timeout reached" 
                if updateErr := s.db.MarkFailed(jobCopy.ID, msg +err.Error()); updateErr != nil {
                    slog.Error("Failed to mark job as timed out", "jobID", jobCopy.ID, "err", updateErr)
                }
            } else if errors.Is(err, context.Canceled) {
                // This means either the batch was canceled or the scheduler is shutting down
				msg := "schedular ordered to stop" 
                if updateErr := s.db.MarkFailed(jobCopy.ID, msg + err.Error()); updateErr != nil {
                    slog.Error("Failed to mark job as interrupted", "jobID", jobCopy.ID, "err", updateErr)
                }
                slog.Info("Job interrupted", "jobID", jobCopy.ID)
            } else {
                if updateErr := s.db.MarkFailed(jobCopy.ID, err.Error()); updateErr != nil {
                    slog.Error("Failed to mark job as failed", "jobID", jobCopy.ID, "err", updateErr)
                }
            }
            
            return err
        })
    }
    
    // Wait for all jobs in this batch to complete or for the parent context to be canceled
    if err := g.Wait(); err != nil {
        if errors.Is(err, context.Canceled) {
            slog.Info("Job batch interrupted due to scheduler shutdown")
        } else {
            slog.Error("Error executing batch jobs", "err", err)
        }
    }

    if len(jobs) > 0 {
        slog.Info("Finished processing claimed jobs", "success", processed, "total", len(jobs))
    }
}

// TODO

func executeJobWithContext(ctx context.Context, job queue.Job) error {
    // Check context before we even start
    if ctx.Err() != nil {
        return ctx.Err()
    }

    // Your job execution code here
    // Regularly check ctx.Done() for long-running operations:
    
    select {
    case <-ctx.Done():
        return ctx.Err() // Return the specific context error (timeout or cancellation)
    default:
        // Proceed with execution
    }
    
    // Simulate work
    time.Sleep(2 * time.Second)
    
    return nil
}

func executeJob(job queue.Job) error {
	slog.Info("Executing job",
		"job_type", job.JobType,
		"payload", job.Payload,
		"status", job.Status,
		"attempts", job.Attempts,
		"maxAttempts", job.MaxAttempts,
	)

	// Simulate job execution
	time.Sleep(2 * time.Second)

	//slog.Info("Completed job", "jobID", job.ID, "jobType", job.JobType)
	return nil
}

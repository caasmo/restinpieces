package job

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"runtime"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/config"
)

const (
	DefaultConcurrencyMultiplier = 2
)

// Scheduler handles scheduled jobs
type Scheduler struct {
	// cfg contains the scheduler configuration including interval and max jobs per tick
	cfg           config.Scheduler
	
	// db is the database connection used to fetch and update jobs
	db            db.Db
	
	// eg is an errgroup.Group used to manage and track running jobs
	// It provides synchronization and error propagation for concurrent job execution
	eg            *errgroup.Group
	
	// ctx is the context used to control the scheduler's lifecycle
	// It allows graceful shutdown when Stop() is called from outside.
	// The context is passed to all job execution goroutines.
	ctx           context.Context
	
	// cancel is the CancelFunc associated with ctx
	// It is called in the Stop method to initiate shutdown of the scheduler
	// and all running jobs.
	cancel        context.CancelFunc
	
	// shutdownDone is a channel that will be closed when the scheduler
	// has completely shut down and all jobs have finished.
	// Used to signal completion of the shutdown process.
	shutdownDone  chan struct{}
}

// NewScheduler creates a new scheduler
func NewScheduler(cfg config.Scheduler, db db.Db) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Calculate concurrency limit based on multiplier and CPU cores
	concurrency := runtime.NumCPU() * cfg.ConcurrencyMultiplier
	
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	
	return &Scheduler{
		cfg:          cfg,
		eg:           g,
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
				if err := s.eg.Wait(); err != nil {
					slog.Error("Error waiting for jobs to complete", "err", err)
				}
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

// processJobs checks for pending and failed jobs and executes them
func (s *Scheduler) processJobs() {
	// Get jobs up to configured limit per tick
	jobs, err := s.db.GetJobs(s.cfg.MaxJobsPerTick)
	if err != nil {
		slog.Error("Failed to fetch jobs", "err", err)
		return
	}

	slog.Info("Processing jobs", "count", len(jobs))

	var processed int
	for _, job := range jobs {
		jobCopy := job // Create a copy to avoid closure issues
		s.eg.Go(func() error {
			err := executeJob(jobCopy)
			if err == nil {
				processed++
			}
			return err
		})
	}

	if len(jobs) > 0 {
		slog.Info("Jobs processed", "success", processed, "total", len(jobs))
	}
}

func executeJob(job queue.QueueJob) error {
    slog.Info("Executing job",
    "payload", job.Payload,
    "status", job.Status,
    "attempts", job.Attempts,
    "maxAttempts", job.MaxAttempts,
    "scheduledFor", job.ScheduledFor,
)

	// Simulate job execution
	time.Sleep(2 * time.Second)

	//slog.Info("Completed job", "jobID", job.ID, "jobType", job.JobType)
	return nil
}


package job

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"time"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/config"
)

// Scheduler handles scheduled jobs
type Scheduler struct {
	// interval specifies how often the scheduler should check for new jobs
	interval      time.Duration
	db            db.Db
	
	// eg is an errgroup.Group used to manage and track running jobs
	eg            *errgroup.Group
	
	// ctx is the context used to control the scheduler's lifecycle
	// It allows graceful shutdown when Stop() is called from outside.
	ctx           context.Context
	
	// cancel is the CancelFunc associated with ctx
	// is called in the Stop method to start the process of shutdown of the start goroutine
	cancel        context.CancelFunc
	
	// shutdownDone is a channel that will be closed when the scheduler
	// has completely shut down and all jobs have finished
	shutdownDone  chan struct{}
}

// NewScheduler creates a new scheduler
func NewScheduler(cfg Config.Scheduler, db db.Db) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	
	return &Scheduler{
		interval:      cfg.Interval,
		eg:            g,
		ctx:           ctx,
		cancel:        cancel,
		db:            db,
		shutdownDone:  make(chan struct{}),
	}
}

// Start begins the job scheduler operation by creting a long runnig goroutine
// that will create gorotines to handle backend jobs
func (s *Scheduler) Start() {
	go func() {
		slog.Info("Starting job scheduler", "interval", s.interval)
		ticker := time.NewTicker(s.interval)
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
	// Get up to 100 jobs at a time
	jobs, err := s.db.GetJobs(100)
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
	slog.Info("Executing job", "jobID", job.ID, "jobType", job.JobType)
	// Simulate job execution
	time.Sleep(2 * time.Second)
	slog.Info("Completed job", "jobID", job.ID, "jobType", job.JobType)
	return nil
}


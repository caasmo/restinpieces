package job

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Scheduler handles scheduled jobs
type Scheduler struct {
	interval time.Duration
	eg       *errgroup.Group
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{} // Channel to signal completion
}

// NewScheduler creates a new scheduler
func NewScheduler(interval time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	
	return &Scheduler{
		interval: interval,
		eg:       g,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
}

// Start begins the job scheduler operation
func (s *Scheduler) Start() {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-s.ctx.Done():
				log.Println("Job scheduler received shutdown signal")
				// Wait for all jobs to complete
				if err := s.eg.Wait(); err != nil {
					log.Printf("Error waiting for jobs to complete: %v", err)
				}
				close(s.done) // Signal that scheduler has completely shut down
				return
			case <-ticker.C:
				s.processJobs()
			}
		}
	}()
}

// StopWithContext signals the scheduler to stop and waits for all jobs to complete
// or the context to be canceled, whichever comes first
func (s *Scheduler) StopWithContext(ctx context.Context) error {
	log.Println("Stopping job scheduler")
	s.cancel()
	
	// Wait for either scheduler completion or context timeout
	select {
	case <-s.done:
		log.Println("Job scheduler stopped gracefully")
		return nil
	case <-ctx.Done():
		log.Println("Job scheduler shutdown timed out")
		return ctx.Err()
	}
}

// processJobs checks for pending jobs and executes them
func (s *Scheduler) processJobs() {
	// This would be replaced with actual database lookup logic
	pendingJobs := fetchPendingJobs()
	
	for _, job := range pendingJobs {
		jobCopy := job // Create a copy to avoid closure issues
		s.eg.Go(func() error {
			return executeJob(jobCopy)
		})
	}
}

// Mock functions for demonstration
func fetchPendingJobs() []string {
	// In a real implementation, this would query your database
	return []string{"job1", "job2"}
}

func executeJob(jobID string) error {
	log.Printf("Executing job: %s", jobID)
	// Simulate job execution
	time.Sleep(2 * time.Second)
	log.Printf("Completed job: %s", jobID)
	return nil
}


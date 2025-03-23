package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/caasmo/restinpieces/queue"
)

// JobExecutor defines the interface for executing jobs
type JobExecutor interface {
	Execute(ctx context.Context, job queue.Job) error
}

// DefaultExecutor is our concrete implementation of JobExecutor
type DefaultExecutor struct {
	registry map[string]JobHandler // Maps job types to handlers
}

// JobHandler processes a specific type of job
type JobHandler interface {
	Handle(ctx context.Context, job queue.Job) error
}

// NewExecutor creates an executor with the given handlers
func NewExecutor(handlers map[string]JobHandler) *DefaultExecutor {
	return &DefaultExecutor{
		registry: handlers,
	}
}

// Execute implements the JobExecutor interface
func (e *DefaultExecutor) Execute(ctx context.Context, job queue.Job) error {

	slog.Info("executor: received job",
		"job_type", job.JobType,
		"attempt", job.Attempts,
		"payload", string(job.Payload),
	)

	handler, exists := e.registry[job.JobType]
	if !exists {
		err := fmt.Errorf("no handler registered for job type: %s", job.JobType)
		slog.Error("executor: job type not found", 
			"job_type", job.JobType,
			"error", err)
		return err
	}

	return handler.Handle(ctx, job)
}


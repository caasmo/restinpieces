package executor

import (
	"context"
	"fmt"

	"github.com/caasmo/restinpieces/db" // Changed import
)

// JobExecutor defines the interface for executing jobs and registering handlers.
type JobExecutor interface {
	Execute(ctx context.Context, job db.Job) error // Changed to db.Job
	Register(jobType string, handler JobHandler)
}

// DefaultExecutor is our concrete implementation of JobExecutor
type DefaultExecutor struct {
	registry map[string]JobHandler // Maps job types to handlers
}

// JobHandler processes a specific type of job
type JobHandler interface {
	Handle(ctx context.Context, job db.Job) error // Changed to db.Job
}

// NewExecutor creates an executor with the given handlers.
// If handlers is nil, an empty map will be initialized for the registry.
func NewExecutor(handlers map[string]JobHandler) *DefaultExecutor {
	if handlers == nil {
		handlers = make(map[string]JobHandler)
	}
	return &DefaultExecutor{
		registry: handlers,
	}
}

// Register adds a new handler for a specific job type.
// It overwrites any existing handler for the same job type.
func (e *DefaultExecutor) Register(jobType string, handler JobHandler) {
	// TODO: Add logging? Potentially return an error if overwriting?
	// For now, simple replacement.
	e.registry[jobType] = handler
}

// Execute implements the JobExecutor interface
func (e *DefaultExecutor) Execute(ctx context.Context, job db.Job) error { // Changed to db.Job

	handler, exists := e.registry[job.JobType]
	if !exists {
		err := fmt.Errorf("no handler registered for job type: %s", job.JobType)
		return err
	}

	return handler.Handle(ctx, job)
}

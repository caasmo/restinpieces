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
	handler, exists := e.registry[job.JobType]
	if !exists {
		return fmt.Errorf("no handler registered for job type: %s", job.JobType)
	}

	slog.Info("Executing job",
		"job_id", job.ID,
		"job_type", job.JobType,
		"attempt", job.Attempts,
	)

	return handler.Handle(ctx, job)
}

// EmailVerificationHandler handles email verification jobs
type EmailVerificationHandler struct {
	mailer *mail.Mailer
}

// NewEmailVerificationHandler creates a new handler for email verification jobs
func NewEmailVerificationHandler(mailer *mail.Mailer) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		mailer: mailer,
	}
}

// Handle implements JobHandler for email verification
func (h *EmailVerificationHandler) Handle(ctx context.Context, job queue.Job) error {
	var payload queue.PayloadEmailVerification
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse email verification payload: %w", err)
	}

	return h.mailer.SendVerificationEmail(ctx, payload.Email, fmt.Sprintf("%d", job.ID))
}

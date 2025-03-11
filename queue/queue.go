package queue

import (
	"encoding/json"
	"time"
)

// QueueJob represents a job in the processing queue
type QueueJob struct {
	ID           int64           `json:"id"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Attempts     int             `json:"attempts"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	ScheduledFor time.Time       `json:"scheduled_for"`
	LockedBy     string          `json:"locked_by,omitempty"`
	LockedAt     time.Time       `json:"locked_at,omitempty"`
	CompletedAt  time.Time       `json:"completed_at,omitempty"`
	LastError    string          `json:"last_error,omitempty"`
}

// PayloadEmailVerification contains the email verification details
type PayloadEmailVerification struct {
	Email string `json:"email"`
}

// Job types
const (
	JobTypeEmailVerification = "job_type_email_verification"
)

// Job statuses
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

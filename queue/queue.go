package queue

import (
	"encoding/json"
	"time"
)

// Job represents a job in the processing queue
type Job struct {
	ID           int64           `json:"id"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Attempts     int             `json:"attempts"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	ScheduledFor time.Time       `json:"scheduled_for"`
	LockedBy     string          `json:"-"` // deprecated, marked as ignored in JSON
	LockedAt     time.Time       `json:"locked_at,omitempty"`
	CompletedAt  time.Time       `json:"completed_at,omitempty"`
	LastError    string          `json:"last_error,omitempty"`
}

// PayloadEmailVerification contains the email verification details
type PayloadEmailVerification struct {
	Email string `json:"email"`
}

type PayloadPasswordReset struct {
	Email string `json:"email"`
}

// Job types
const (
	JobTypeEmailVerification = "job_type_email_verification"
	JobTypePasswordReset     = "job_type_password_reset"
)

// Job statuses
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// CoolDownBucket calculates which time bucket the current time falls into based on the duration period.
// It returns an integer representing the number of complete duration periods since the Unix epoch (January 1, 1970 UTC).
//
// This function is useful for implementing rate limiting and cooldown periods by grouping requests
// into fixed time windows. For example:
// - With a 1 hour duration, CoolDownBucket returns same value for all times within same hour
// - With a 5 minute duration, CoolDownBucket groups times into 5 minute buckets
//
// The bucket number increases monotonically over time and can be used as a cache key for rate limiting.
// Multiple requests within the same duration period will get same bucket number.
//
// Parameters:
// - duration: The fixed time window size to bucket time into (e.g. time.Hour, 5*time.Minute)
// - t: The time to calculate the bucket for (defaults to time.Now() if nil)
//
// Returns:
// int: The bucket number, calculated as floor(t.Unix() / duration)
//
// Errors:
// - Panics if duration is zero or negative to prevent undefined behavior
func CoolDownBucket(duration time.Duration, t time.Time) int {
	if duration <= 0 {
		panic("duration must be positive")
	}

	return int(t.Unix() / int64(duration.Seconds()))
}

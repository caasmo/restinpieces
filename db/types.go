package db

import (
	"encoding/json"
	"time"
)

// User represents a user from the database.
// Timestamps (Created and Updated) use RFC3339 format in UTC timezone.
// Example: "2024-03-07T15:04:05Z"
type User struct {
	ID    string
	Email string
	Name  string
	// Non empty password means password authentication is active
	// Password can be empty for passwordless methods like oauth2, otp over email...
	Password string
	Avatar   string
	Created  time.Time
	Updated  time.Time
	Verified bool
	//deprecated
	// ExternalAuth identifies authentication methods (password authentication excluded)
	// Example of methods are "oauth2", "otp".
	// the structure is a comma separated string
	// in future a colon separated string (not implmented) could be used for mfa
	//
	// The only reason for this field is the use case of a user having password and oauth2 login with the same email,
	// if the user request a change of email, and after that tries to  log with the
	// the old email a new user is created which may surprise the user.
	// having this field, we now it has two auth methods and we can remember the user before changing email.
	//ExternalAuth    string
	Oauth2          bool
	EmailVisibility bool
}

// DbApp is an interface combining the required DB roles for the application.
// The concrete DB implementation (e.g., *crawshaw.Db or *zombiezen.Db) must satisfy this interface.

// Job represents a job in the processing queue
type Job struct {
	ID           int64           `json:"id"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`       // Unique payload part
	PayloadExtra json.RawMessage `json:"payload_extra"` // Non-unique payload part
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
	Recurrent    bool            `json:"recurrent"`
	Interval     time.Duration   `json:"interval"` // Go duration
}

// PayloadEmailVerification contains the email verification details
type PayloadEmailVerification struct {
	Email string `json:"email"`
	// CooldownBucket is the time bucket number calculated from the current time divided by the cooldown duration.
	// This provides a basic rate limiting mechanism where only one email verification request is allowed per time bucket.
	// The bucket number is calculated as: floor(current Unix time / cooldown duration in seconds)
	CooldownBucket int `json:"cooldown_bucket"`
}


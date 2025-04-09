package db

import (
	"errors"
	"github.com/caasmo/restinpieces/queue"
	"time"
)

// DbAuth defines database operations related to users and authentication.
type DbAuth interface {
	GetUserByEmail(email string) (*User, error)
	GetUserById(id string) (*User, error)
	CreateUserWithPassword(user User) (*User, error)
	CreateUserWithOauth2(user User) (*User, error)
	VerifyEmail(userId string) error
	UpdatePassword(userId string, newPassword string) error
	UpdateEmail(userId string, newEmail string) error
}

// DbQueue defines database operations related to the job queue.
type DbQueue interface {
	InsertJob(job queue.Job) error
	// GetJobs(limit int) ([]*queue.Job, error) // Removed as Claim is usually preferred
	Claim(limit int) ([]*queue.Job, error)
	MarkCompleted(jobID int64) error
	MarkFailed(jobID int64, errMsg string) error
}

// DbConfig defines database operations related to configuration.
type DbConfig interface {
	// GetConfig returns the TOML serialized configuration from the database
	GetConfig() (string, error)
}

// DbApp is an interface combining the required DB roles for the application.
// The concrete DB implementation (e.g., *crawshaw.Db or *zombiezen.Db) must satisfy this interface.
type AcmeCert struct {
	Key         []byte // Private key data
	Certificate []byte // Certificate data
}

type DbAcme interface {
	// Get retrieves the current ACME certificate
	Get() (*AcmeCert, error)
}

type DbApp interface {
	DbAuth
	DbQueue
	DbConfig
	DbAcme
}

// DbLifecycle interface removed.

// TimeFormat converts a time.Time to RFC3339 string in UTC.
// This should be used when sending time values to SQLite since it doesn't have
// a native datetime type. All timestamps in the database should use this format.
// Example: "2024-03-11T15:04:05Z"
func TimeFormat(tt time.Time) string {
	return tt.UTC().Format(time.RFC3339)
}

// TimeParse parses a RFC3339 string into a time.Time.
// This should be used when reading timestamps from SQLite to convert them
// back to time.Time values. Returns an error if the input string is not
// in RFC3339 format.
func TimeParse(s string) (time.Time, error) {
	// Handle empty strings gracefully, returning zero time and no error,
	// as some DB fields might be nullable/empty timestamps.
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

var (
	ErrMissingFields    = errors.New("missing required fields")
	ErrConstraintUnique = errors.New("unique constraint violation")
	ErrUserNotFound     = errors.New("user not found") // Added for clarity
)

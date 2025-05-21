package db

import (
	"errors"
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
	InsertJob(job Job) error
	Claim(limit int) ([]*Job, error)
	MarkCompleted(jobID int64) error
	MarkFailed(jobID int64, errMsg string) error
	MarkRecurrentCompleted(completedJobID int64, newJob Job) error
}

// DbConfig defines database operations related to configuration.
type DbConfig interface {
	// GetConfig retrieves encrypted config content and format by exact scope and generation offset
	// scope must be provided, generation is passed through directly
	// generation 0 = latest, 1 = previous, etc.
	GetConfig(scope string, generation int) ([]byte, string, error)
	// InsertConfig inserts a new configuration content blob for a given scope.
	InsertConfig(scope string, contentData []byte, format string, description string) error
}

type DbApp interface {
	DbAuth
	DbQueue
	DbConfig
}

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

package db

import (
	"errors"
	"github.com/caasmo/restinpieces/queue"
	"time"
)

type Db interface {
	Close()
	GetById(id int64) int
	Insert(value int64)
	InsertWithPool(value int64)
	GetUserByEmail(email string) (*User, error)
	GetUserById(id string) (*User, error)
	CreateUserWithPassword(user User) (*User, error)
	CreateUserWithOauth2(user User) (*User, error)
	InsertJob(job queue.QueueJob) error
	GetJobs(limit int) ([]queue.QueueJob, error)
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
	return time.Parse(time.RFC3339, s)
}

var (
	ErrMissingFields    = errors.New("missing required fields")
	ErrConstraintUnique = errors.New("unique constraint violation")
)

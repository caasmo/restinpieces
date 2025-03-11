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
	CreateUser(user User) (*User, error)
	InsertQueueJob(job queue.QueueJob) error
}

// User represents a user from the database.
// Timestamps (Created and Updated) use RFC3339 format in UTC timezone.
// Example: "2024-03-07T15:04:05Z"
type User struct {
	ID       string
	Email    string
	Name     string
	Password string
	Avatar   string
	Created  string
	Updated  string
	Verified bool
	TokenKey string
}

// TimeFormat converts a time.Time to RFC3339 string in UTC
func TimeFormat(tt time.Time) string {
	return tt.UTC().Format(time.RFC3339)
}

// TimeNow returns the current time formatted in UTC RFC3339
func TimeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// TimeParse parses a RFC3339 string into a time.Time
func TimeParse(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

var (
	ErrMissingFields    = errors.New("missing required fields")
	ErrConstraintUnique = errors.New("unique constraint violation")
)

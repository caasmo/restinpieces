package db

import "errors"

// User represents a user from the database.
// Timestamps (Created and Updated) use RFC3339 format in UTC timezone.
// Example: "2024-03-07T15:04:05Z"
type User struct {
	ID        string
	Email     string
	Name      string
	Password  string
	Avatar    string
	Created   string
	Updated   string
	Verified  bool
	TokenKey  string
}

var (
	ErrMissingFields    = errors.New("missing required fields")
	ErrConstraintUnique = errors.New("unique constraint violation")
)

type Db interface {
	Close()
	GetById(id int64) int
	Insert(value int64)
	InsertWithPool(value int64)
	GetUserByEmail(email string) (*User, error)
	CreateUser(user User) (*User, error)
}

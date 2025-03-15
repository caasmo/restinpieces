package db

import "time"

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
	// ExternalAuth identifies authentication methods (password authentication excluded)
	// Example of methods are "oauth2", "otp".
	// the structure is a comma separated string
	// in future a colon separated string (not implmented) could be used for mfa
	ExternalAuth    string
	EmailVisibility bool
}

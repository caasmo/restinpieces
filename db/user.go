package db

import "time"

// User represents a user from the database.
// Timestamps (Created and Updated) use RFC3339 format in UTC timezone.
// Example: "2024-03-07T15:04:05Z"
type User struct {
	ID           string
	Email        string
	Name         string
	Password     string
	Avatar       string
	Created      time.Time
	Updated      time.Time
	Verified     bool
	// ExternalAuth identifies the authentication provider method, excluding password
	// (e.g. "google", "github"). Empty string indicates password authentication.
	ExternalAuth string
}

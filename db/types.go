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

// AcmeCert represents an ACME certificate record from the database.
// Timestamps use RFC3339 format in UTC timezone.
type AcmeCert struct {
	ID                   int64     // Primary Key
	Identifier           string    // Unique identifier (e.g., primary domain)
	Domains              string    // JSON array of domains
	CertificateChain     string    // PEM encoded certificate chain
	PrivateKey           string    // PEM encoded private key
	IssuedAt             time.Time // UTC timestamp
	ExpiresAt            time.Time // UTC timestamp
	LastRenewalAttemptAt time.Time // UTC timestamp (zero time if null/not set)
	CreatedAt            time.Time // UTC timestamp
	UpdatedAt            time.Time // UTC timestamp
}

package core

import (
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"
)

// ValidateEmail checks if an email address is valid according to RFC 5322
// Returns nil if valid, or an error describing why the email is invalid
func ValidateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	return nil
}


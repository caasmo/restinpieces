package app

import (
	"net/mail"
)

// ValidateEmail checks if an email address is valid according to RFC 5322
func ValidateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

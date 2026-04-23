package core

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
)

// Validator defines an interface for request validation operations
type Validator interface {
	// ContentType checks if the request's Content-Type matches the allowed type
	ContentType(r *http.Request, allowedType string) (jsonResponse, error)

	// Email checks if an email address is valid according to RFC 5321/5322.
	// It rejects display-name formats ("Name <addr>"), enforces length limits,
	// and ensures the domain has a valid structure.
	Email(email string) error
}

// DefaultValidator implements the Validator interface
type DefaultValidator struct{}

// NewValidator creates a new DefaultValidator instance
func NewValidator() Validator {
	return &DefaultValidator{}
}

// ContentType checks if the request's Content-Type matches the allowed type.
// Returns:
// - error (always "Invalid content type" for security)
// - precomputed jsonResponse for error cases
// Uses http.StatusUnsupportedMediaType (415) for invalid content types as per HTTP spec.
func (v *DefaultValidator) ContentType(r *http.Request, allowedType string) (jsonResponse, error) {
	errInvalidType := errors.New("invalid content type")
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return errorInvalidContentType, errInvalidType
	}

	// Handle cases where Content-Type includes charset or other parameters
	// e.g. "application/json; charset=utf-8"
	mediaType := strings.Split(contentType, ";")[0]
	mediaType = strings.TrimSpace(mediaType)

	if mediaType != allowedType {
		return errorInvalidContentType, errInvalidType
	}

	return jsonResponse{}, nil
}

// Email checks if an email address is valid according to RFC 5321/5322.
// It rejects display-name formats ("Name <addr>"), enforces length limits,
// and ensures the domain has a valid structure.
func (v *DefaultValidator) Email(email string) error {
	// RFC 5321: max total length is 254 characters
	if len(email) > 254 {
		return fmt.Errorf("email address exceeds 254 character limit")
	}

	// mail.ParseAddress accepts "Display Name <user@host>" — we reject that.
	// A bare email must round-trip: parsed address must equal the input exactly.
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	if addr.Address != email {
		return fmt.Errorf("invalid email format: display name or angle brackets are not allowed")
	}

	// Split into local and domain parts for further checks.
	// mail.ParseAddress already guarantees exactly one '@' at this point.
	at := strings.LastIndex(email, "@")
	localPart := email[:at]
	domain := email[at+1:]

	// RFC 5321: local part must not exceed 64 characters
	if len(localPart) > 64 {
		return fmt.Errorf("email local part exceeds 64 character limit")
	}

	// Domain must contain at least one dot and no empty labels.
	// Rejects: "user@localhost", "user@.com", "user@com."
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("email domain must contain at least one dot")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return fmt.Errorf("email domain contains an empty label")
		}
	}

	return nil
}

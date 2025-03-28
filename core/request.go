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

// getClientIP extracts the client IP address from the request, handling proxies via X-Forwarded-For header
func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Handle error potentially, or use RemoteAddr directly if no port
		ip = r.RemoteAddr
	}
	// Consider X-Forwarded-For header if behind a proxy
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Use the first IP in the list
		parts := strings.Split(forwarded, ",")
		ip = strings.TrimSpace(parts[0])
	}
	return ip
}

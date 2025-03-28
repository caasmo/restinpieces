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

// GetClientIP extracts the client IP address from the request, handling proxies via configured header
func (a *App) GetClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Handle error potentially, or use RemoteAddr directly if no port
		ip = r.RemoteAddr
	}

	if a.Config != nil && a.Config.Server.ClientIpProxyHeader != "" {
		if forwarded := r.Header.Get(a.Config.Server.ClientIpProxyHeader); forwarded != "" {
			// Use the first IP in the list if header contains multiple
			parts := strings.Split(forwarded, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	return ip
}


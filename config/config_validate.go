package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func Validate(cfg *Config) error {
	if err := validateServer(&cfg.Server); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}
	// Add calls to other validation functions here later
	return nil
}

// validateServer checks the Server configuration section.
// It ensures the Addr field is not empty and contains a valid host:port or :port format.
// If only a port is provided (e.g., ":8080"), it defaults the host to "localhost".
//
// Allowed formats:
//   - "host:port" (e.g., "example.com:8080", "127.0.0.1:8080", "[::1]:8080")
//   - ":port"     (e.g., ":8080" becomes "localhost:8080")
//
// The port part is mandatory.
func validateServer(server *Server) error {
	if err := validateServerAddr(server); err != nil {
		return err // Error already includes context
	}

	// Always validate RedirectPort if it's set, regardless of EnableTLS.
	// validateServerRedirectPort handles the empty case correctly.
	if err := validateServerRedirectPort(server); err != nil {
		return err // Error already includes context
	}

	// Add calls to validate other Server fields here if needed

	return nil
}

// validateServerAddr checks the Server.Addr field.
// It ensures the format is host:port or :port, defaulting host to localhost if needed.
// It modifies server.Addr in place if defaulting occurs.
func validateServerAddr(server *Server) error {
	if server.Addr == "" {
		return fmt.Errorf("server address (Addr) cannot be empty")
	}

	host, port, err := net.SplitHostPort(server.Addr)
	if err != nil {
		// Check if it's just a port (e.g., ":8080")
		if strings.HasPrefix(server.Addr, ":") {
			port = strings.TrimPrefix(server.Addr, ":")
			host = "localhost" // Default host
		} else {
			return fmt.Errorf("invalid server address format '%s': %w", server.Addr, err)
		}
	}

	if port == "" {
		return fmt.Errorf("server address '%s' must include a port", server.Addr)
	}

	// Reconstruct the address with the defaulted host if necessary
	server.Addr = net.JoinHostPort(host, port)

	// Basic check: Ensure port is numeric (net.SplitHostPort doesn't guarantee this fully)
	if _, err := net.LookupPort("tcp", port); err != nil {
		return fmt.Errorf("invalid port '%s' in server address '%s': %w", port, server.Addr, err)
	}

	// Note: Host validation (is it a valid domain/IP?) is complex and often
	// better left to the net.Listen call during server startup.
	return nil
}

// validateServerRedirectPort checks the Server.RedirectPort field value.
// It allows an empty string "" (meaning no redirect server).
// If non-empty, it ensures the value is a valid port number (1-65535)
// and does not contain ":". Port "0" is invalid.
func validateServerRedirectPort(server *Server) error {
	portStr := server.RedirectPort

	// Empty means no redirect server, which is valid configuration.
	if portStr == "" {
		return nil
	}

	// If set, it must not contain ":" (we only want the port number)
	if strings.Contains(portStr, ":") {
		return fmt.Errorf("invalid RedirectPort '%s': must be a port number, not an address", portStr)
	}

	// If set, it must be a valid port number
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid RedirectPort '%s': must be a number: %w", portStr, err)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid RedirectPort '%d': port number must be between 1 and 65535", portNum)
	}

	return nil
}

package config

import (
	"fmt"
	"net"
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

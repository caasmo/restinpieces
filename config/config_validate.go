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
		return err
	}

	if err := validateServerRedirectAddr(server); err != nil {
		return err
	}

	if err := validateServerTLS(server); err != nil {
		return err
	}

	return nil
}

func sanitizeAddrEmptyHost(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

// validateServerAddr checks the Server.Addr field.
// It ensures the format is host:port or :port.
// Returns an error if the address is invalid.
func validateServerAddr(server *Server) error {
	if server.Addr == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	// Split into host and port components
	_, port, err := net.SplitHostPort(server.Addr)
	if err != nil {
		return fmt.Errorf("invalid server address format '%s': %w", server.Addr, err)
	}

	// Validate the port component
	if err := validateServerPort(port); err != nil {
		return fmt.Errorf("invalid server port in address '%s': %w", server.Addr, err)
	}

	return nil
}

func validateServerRedirectAddr(server *Server) error {
	// If RedirectPort is empty, no redirect server is configured (valid case)
	if server.RedirectAddr == "" {
		return nil
	}

	// Construct the redirect address from the main server's host and redirect port
	_, port, err := net.SplitHostPort(server.RedirectAddr)
	if err != nil {
		return fmt.Errorf("failed to parse host from server address '%s': %w", server.Addr, err)
	}

	// Validate the port component
	if err := validateServerPort(port); err != nil {
		return fmt.Errorf("invalid server port in address '%s': %w", server.Addr, err)
	}

	return nil
}

// validateServerTLS checks that CertData and KeyData are present if TLS is enabled.
func validateServerTLS(server *Server) error {
	if !server.EnableTLS {
		return nil // No validation needed if TLS is disabled
	}

	// If TLS is enabled, CertData and KeyData must not be empty.
	// CertFile and KeyFile are ignored if CertData/KeyData are present.
	if server.CertData == "" {
		return fmt.Errorf("server.cert_data cannot be empty when TLS is enabled")
	}
	if server.KeyData == "" {
		return fmt.Errorf("server.key_data cannot be empty when TLS is enabled")
	}

	return nil
}

func validateServerPort(portStr string) error {
	// Empty means no redirect server, which is valid configuration.
	if portStr == "" {
		return nil
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

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
	if err := validateJwt(&cfg.Jwt); err != nil {
		return fmt.Errorf("jwt config validation failed: %w", err)
	}
	if err := validateSmtp(&cfg.Smtp); err != nil {
		return fmt.Errorf("smtp config validation failed: %w", err)
	}
	if err := validateAcme(&cfg.Acme); err != nil {
		return fmt.Errorf("acme config validation failed: %w", err)
	}
	if err := validateOAuth2Providers(cfg.OAuth2Providers); err != nil {
		return fmt.Errorf("oauth2 providers validation failed: %w", err)
	}
	return nil
}

func validateOAuth2Providers(providers map[string]OAuth2Provider) error {
	for name, provider := range providers {
		if provider.RedirectURL == "" && provider.RedirectURLPath == "" {
			return fmt.Errorf("oauth2 provider '%s' must have either redirect_url or redirect_url_path configured", name)
		}
	}
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

func validateJwt(jwt *Jwt) error {
	if jwt.AuthSecret == "" {
		return fmt.Errorf("jwt.auth_secret cannot be empty")
	}
	if jwt.VerificationEmailSecret == "" {
		return fmt.Errorf("jwt.verification_email_secret cannot be empty")
	}
	if jwt.PasswordResetSecret == "" {
		return fmt.Errorf("jwt.password_reset_secret cannot be empty")
	}
	if jwt.EmailChangeSecret == "" {
		return fmt.Errorf("jwt.email_change_secret cannot be empty")
	}
	return nil
}

func validateSmtp(smtp *Smtp) error {
	if !smtp.Enabled {
		return nil // No validation needed if SMTP is disabled
	}
	if smtp.Host == "" {
		return fmt.Errorf("smtp.host cannot be empty when enabled")
	}
	if smtp.Port == 0 {
		return fmt.Errorf("smtp.port cannot be 0 when enabled")
	}
	if smtp.FromAddress == "" {
		return fmt.Errorf("smtp.from_address cannot be empty when enabled")
	}
	if smtp.Username == "" {
		return fmt.Errorf("smtp.username cannot be empty when enabled")
	}
	if smtp.Password == "" {
		return fmt.Errorf("smtp.password cannot be empty when enabled")
	}
	return nil
}

func validateAcme(acme *Acme) error {
	if !acme.Enabled {
		return nil // No validation needed if ACME is disabled
	}
	if acme.Email == "" {
		return fmt.Errorf("acme.email cannot be empty when enabled")
	}
	if len(acme.Domains) == 0 {
		return fmt.Errorf("acme.domains cannot be empty when enabled")
	}
	if acme.DNSProvider == "" {
		return fmt.Errorf("acme.dns_provider cannot be empty when enabled")
	}
	if acme.DNSProvider == "cloudflare" && acme.CloudflareApiToken == "" {
		return fmt.Errorf("acme.cloudflare_api_token cannot be empty when dns_provider is 'cloudflare'")
	}
	if acme.AcmePrivateKey == "" {
		return fmt.Errorf("acme.acme_private_key cannot be empty when enabled")
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

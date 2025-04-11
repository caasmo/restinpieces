package proxy

import (
	"log/slog"
	"net" // Keep for potential future IP checks, or remove if definitely not needed
	"net/http"

	// "github.com/caasmo/restinpieces/assets" // No longer needed for simple text response
	"github.com/caasmo/restinpieces/core"
)

// MaintenanceMiddleware handles serving a maintenance page based on configuration.
type MaintenanceMiddleware struct {
	app    *core.App // Use App to access config
	logger *slog.Logger
}

// NewMaintenanceMiddleware creates a new maintenance middleware instance.
// It requires the core App instance to access configuration and IP detection logic.
func NewMaintenanceMiddleware(app *core.App, logger *slog.Logger) *MaintenanceMiddleware {
	if app == nil {
		panic("app cannot be nil for MaintenanceMiddleware")
	}
	if logger == nil {
		panic("logger cannot be nil for MaintenanceMiddleware")
	}
	// No longer need to check embedded page size
	logger.Debug("Maintenance middleware initialized")
	return &MaintenanceMiddleware{
		app:    app,
		logger: logger.With("middleware", "maintenance"), // Add context to logger
	}
}

// Execute wraps the next handler with maintenance mode logic.
func (m *MaintenanceMiddleware) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := m.app.Config() // Get current config snapshot via App
		maintCfg := cfg.Maintenance

		// Check if feature enabled and mode activated
		if maintCfg.Enabled && maintCfg.Activated {
			// IP bypass logic removed for now

			m.logger.Info("Maintenance mode active, serving maintenance text", "path", r.URL.Path)

			// Set headers BEFORE writing status code or body
			// w.Header().Set("Content-Encoding", "gzip") // No longer gzipped
			w.Header().Set("Content-Type", "text/plain; charset=utf-8") // Plain text response
			// Prevent caching of the maintenance page by clients and proxies
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			w.Header().Set("Pragma", "no-cache") // HTTP/1.0 backward compatibility
			w.Header().Set("Expires", "0")       // Proxies
			// Indicate service is temporarily unavailable and suggest retrying later (e.g., 10 minutes)
			w.Header().Set("Retry-After", "600")

			w.WriteHeader(http.StatusServiceUnavailable) // 503 Service Unavailable

			// Write the simple text message
			_, err := w.Write([]byte("Maintenance. Page comes later."))
			if err != nil {
				// Log error, but response headers/status might be already sent
				m.logger.Error("Failed to write maintenance text response body", "error", err)
			}
			return // Stop processing the request here
		}

		// If maintenance mode is not active, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

// isIPAllowed checks if a given client IP matches any of the allowed IPs or CIDR ranges.
// Removed as AllowedIPs feature is temporarily disabled.
/*
func (m *MaintenanceMiddleware) isIPAllowed(clientIP string, allowedEntries []string) bool {
	if len(allowedEntries) == 0 {
		return false // No allowed list means no one is allowed by default
	}

	parsedClientIP := net.ParseIP(clientIP)
	if parsedClientIP == nil {
		m.logger.Warn("Could not parse client IP for maintenance bypass check", "ip", clientIP)
		return false // Cannot check an invalid client IP
	}

	for _, entry := range allowedEntries {
		// Try parsing as CIDR first
		_, ipNet, err := net.ParseCIDR(entry)
		if err == nil {
			// Valid CIDR, check if the client IP is within the range
			if ipNet.Contains(parsedClientIP) {
				return true
			}
		} else {
			// Not a valid CIDR, try direct IP comparison
			allowedIP := net.ParseIP(entry)
			if allowedIP != nil && allowedIP.Equal(parsedClientIP) {
				return true
			}
			// Log if the entry is neither a valid CIDR nor a valid IP? Optional.
			// m.logger.Warn("Invalid entry in maintenance allowed_ips list", "entry", entry)
		}
	}

	return false // IP did not match any allowed entry
}
*/

package proxy

import (
	"log/slog"
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// MaintenanceMiddleware handles serving a maintenance page based on configuration.
type MaintenanceMiddleware struct {
	app    *core.App // Use App to access config and GetClientIP
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
	if len(assets.MaintenancePageGzipped) == 0 {
		// This check helps catch build issues where the embedding failed silently.
		logger.Warn("Embedded maintenance page is empty. Ensure 'assets/maintenance.html.gz' exists and is embedded correctly.")
	} else {
		logger.Debug("Maintenance page loaded", "size_bytes", len(assets.MaintenancePageGzipped))
	}
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
			// Use the App's GetClientIP method


			// Set headers BEFORE writing status code or body
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Prevent caching of the maintenance page by clients and proxies
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			w.Header().Set("Pragma", "no-cache") // HTTP/1.0 backward compatibility
			w.Header().Set("Expires", "0")       // Proxies
			// Indicate service is temporarily unavailable and suggest retrying later (e.g., 10 minutes)
			w.Header().Set("Retry-After", "600")

			w.WriteHeader(http.StatusServiceUnavailable) // 503 Service Unavailable

			_, err := w.Write(assets.MaintenancePageGzipped)
			if err != nil {
				// Log error, but response headers/status might be already sent
				m.logger.Error("Failed to write maintenance page response body", "error", err)
			}
			return // Stop processing the request here
		}

		// If maintenance mode is not active or IP is allowed, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

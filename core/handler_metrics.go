package core

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler serves Prometheus metrics in the standard format
// Endpoint: GET /metrics
// Authenticated: No
// Allowed Mimetype: text/plain
func (a *App) MetricsHandler(w http.ResponseWriter, r *http.Request) {

	// Set proper content type for metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Delegate to Prometheus handler
	h := promhttp.Handler()
	h.ServeHTTP(w, r)
}

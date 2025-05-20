package core

import (
	"net"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler serves Prometheus metrics in the standard format
// Endpoint: GET /metrics
// Authenticated: No
// Allowed Mimetype: text/plain
func (a *App) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if metrics endpoint is enabled
	if !a.Config().Metrics.Enabled {
		WriteJsonError(w, errorNotFound)
		return
	}

	// Get client IP
	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if clientIP == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Check if IP is allowed
	allowed := false
	for _, ip := range a.Config().Metrics.AllowedIPs {
		if ip == clientIP {
			allowed = true
			break
		}
		// Check CIDR ranges if specified
		if strings.Contains(ip, "/") {
			_, cidrNet, err := net.ParseCIDR(ip)
			if err != nil {
				continue
			}
			clientAddr := net.ParseIP(clientIP)
			if clientAddr != nil && cidrNet.Contains(clientAddr) {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		WriteJsonError(w, errorNotFound)
		return
	}

	// Set proper content type for metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Delegate to Prometheus handler
	h := promhttp.Handler()
	h.ServeHTTP(w, r)
}

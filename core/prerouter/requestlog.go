package prerouter

import (
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/core"
)

// RequestLog is middleware that logs HTTP request details
type RequestLog struct {
	app *core.App
}

// NewRequestLog creates a new request logging middleware instance
func NewRequestLog(app *core.App) *RequestLog {
	return &RequestLog{
		app: app,
	}
}

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Execute wraps the next handler with request logging
func (r *RequestLog) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		
		// Create response recorder to capture status code
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		
		// Call next handler
		next.ServeHTTP(rec, req)
		
		// Calculate duration
		duration := time.Since(start)
		
		// Log request details
		r.app.Logger().Info("request",
			"method", req.Method,
			"url", req.URL.String(),
			"status", rec.status,
			"duration", duration.String(),
			"remote_ip", req.RemoteAddr,
			"user_agent", req.UserAgent(),
			"referer", req.Referer(),
			"auth", "", // Empty auth info as requested
		)
	})
}

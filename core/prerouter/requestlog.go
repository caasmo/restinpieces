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
		// Cache common values at package level
		var (
			logType    = slog.String("type", "request")
			emptyAuth  = slog.String("auth", "")
		)

		// More efficient time measurement
		start := time.Now().UnixNano()
		
		// Create response recorder to capture status code
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		
		// Call next handler
		next.ServeHTTP(rec, req)
		
		// Calculate duration in nanoseconds and convert to time.Duration
		duration := time.Duration(time.Now().UnixNano() - start)
		
		// Build log attributes efficiently with cached values
		attrs := make([]any, 0, 15)
		attrs = append(attrs, logType)
		attrs = append(attrs, slog.String("method", req.Method))
		attrs = append(attrs, slog.String("url", req.URL.String()))
		attrs = append(attrs, slog.Int("status", rec.status))
		attrs = append(attrs, slog.String("duration", duration.String()))
		attrs = append(attrs, slog.String("remote_ip", req.RemoteAddr))
		attrs = append(attrs, slog.String("user_agent", req.UserAgent()))
		attrs = append(attrs, slog.String("referer", req.Referer()))
		attrs = append(attrs, emptyAuth)

		// Log request details
		r.app.Logger().Info("", attrs...)
	})
}

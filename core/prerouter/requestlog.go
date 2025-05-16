package prerouter

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/core"
	"log/slog"
)

// RemoteIP returns the normalized IP address from the request
func RemoteIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	parsed, err := netip.ParseAddr(ip)
	if err != nil {
		return ip // fallback to original if parsing fails
	}
	return parsed.StringExpanded()
}

// cutStr limits string length by adding ellipsis if needed
func cutStr(str string, max int) string {
	if len(str) > max {
		return str[:max] + "..."
	}
	return str
}

// Define reasonable max lengths for string fields
const (
	maxURL       = 512
	maxUserAgent = 256
	maxReferer   = 512
	maxRemoteIP  = 64
)

// Cached common log attributes
var (
	logType   = slog.String("type", "request")
	emptyAuth = slog.String("auth", "")
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
		// More efficient time measurement
		start := time.Now().UnixNano()
		
		// Create response recorder to capture status code
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		
		// Call next handler
		next.ServeHTTP(rec, req)
		
		// Calculate duration in nanoseconds and convert to time.Duration
		duration := time.Duration(time.Now().UnixNano() - start)

		// Build log attributes efficiently with cached values and length limits
		attrs := make([]any, 0, 15)
		attrs = append(attrs, logType)
		attrs = append(attrs, slog.String("method", strings.ToUpper(req.Method))) // Ensure uppercase method
		attrs = append(attrs, slog.String("url", cutStr(req.URL.String(), maxURL)))
		attrs = append(attrs, slog.Int("status", rec.status))
		attrs = append(attrs, slog.String("duration", duration.String()))
		attrs = append(attrs, slog.String("remote_ip", cutStr(RemoteIP(req), maxRemoteIP)))
		attrs = append(attrs, slog.String("user_agent", cutStr(req.UserAgent(), maxUserAgent)))
		attrs = append(attrs, slog.String("referer", cutStr(req.Referer(), maxReferer)))
		attrs = append(attrs, emptyAuth)

		// Log request details
		r.app.Logger().Info("", attrs...)
	})
}

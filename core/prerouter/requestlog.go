package prerouter

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"runtime"

	"github.com/caasmo/restinpieces/core"
	"log/slog"
)

const (
	maxBodySize = 1 << 20 // 1MB
)

// RemoteIP returns the normalized IP address from the request
// TODO remove
func RemoteIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	parsed, err := netip.ParseAddr(ip)
	if err != nil {
		return ip // fallback to original if parsing fails
	}
	return parsed.StringExpanded()
}

// runtimeNano provides high-precision timing with better performance than time.Now()
// TODO worth it?
func runtimeNano() int64 {
	var ts int64
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		// Use CPU timestamp counter for supported architectures
		ts = time.Now().UnixNano() // Fallback for now - could use RDTSC on amd64
	} else {
		ts = time.Now().UnixNano()
	}
	return ts
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

// responseRecorder wraps http.ResponseWriter to capture status code.
// Initialized to StatusOK (200) because handlers may:
// 1. Write response body without calling WriteHeader (implicit 200)
// 2. Only call WriteHeader for error cases
// 3. Let the http package set default 200 status
type responseRecorder struct {
	http.ResponseWriter
	status int // initialized to http.StatusOK (200) to handle implicit success cases
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Execute wraps the next handler with request logging
func (r *RequestLog) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.app.Logger().Info("start")

		// Limit request body size
		req.Body = http.MaxBytesReader(w, req.Body, maxBodySize)

		// High-precision, efficient time measurement
		start := runtimeNano()
		
		// Create response recorder initialized to StatusOK (200) to handle implicit success cases
		rec := &responseRecorder{
			ResponseWriter: w,
			status:         http.StatusOK, // default success status
		}
		
		// Call next handler
		next.ServeHTTP(rec, req)
		
		// Calculate duration in nanoseconds and convert to time.Duration
		duration := time.Duration(time.Now().UnixNano() - start)

		// Build log attributes efficiently with cached values and length limits
		attrs := make([]any, 0, 17) // Increased capacity for new fields
		attrs = append(attrs, logType)
		attrs = append(attrs, slog.String("method", strings.ToUpper(req.Method))) // Ensure uppercase method
		attrs = append(attrs, slog.String("path", cutStr(req.URL.RequestURI(), maxURL)))
		attrs = append(attrs, slog.Int("status", rec.status))
		attrs = append(attrs, slog.String("duration", duration.String()))
		attrs = append(attrs, slog.String("remote_ip", cutStr(RemoteIP(req), maxRemoteIP)))
		attrs = append(attrs, slog.String("user_agent", cutStr(req.UserAgent(), maxUserAgent)))
		attrs = append(attrs, slog.String("referer", cutStr(req.Referer(), maxReferer)))
		attrs = append(attrs, slog.String("host", cutStr(req.Host, maxRemoteIP)))
		attrs = append(attrs, slog.String("proto", req.Proto))
		attrs = append(attrs, slog.Int64("content_length", req.ContentLength))
		attrs = append(attrs, emptyAuth)

		// Debug log to verify attributes before sending
		r.app.Logger().Info("preparing request log",
			"attrs_count", len(attrs))

		// Log request with explicit message
		r.app.Logger().Info("http_request", attrs...)

		// Debug log to verify the log was processed
		r.app.Logger().Info("request log sent to batch processor",
			"path", req.URL.Path,
			"status", rec.status)
	})
}

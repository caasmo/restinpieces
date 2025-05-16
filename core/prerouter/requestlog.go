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



// cutStr limits string length by adding ellipsis if needed
func cutStr(str string, max int) string {
	if len(str) > max {
		return str[:max] + "..."
	}
	return str
}

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

		// Limit request body size TODO
		//req.Body = http.MaxBytesReader(w, req.Body, maxBodySize)

		// Simple time measurement
		start := time.Now()
		
		// Create response recorder initialized to StatusOK (200) to handle implicit success cases
		rec := &responseRecorder{
			ResponseWriter: w,
			status:         http.StatusOK, // default success status
		}
		
		// Call next handler
		next.ServeHTTP(rec, req)
		
		// Calculate duration
		duration := time.Since(start)

		// Build log attributes efficiently with cached values and length limits
		attrs := make([]any, 0, 17) // Increased capacity for new fields
		attrs = append(attrs, logType)
		attrs = append(attrs, slog.String("method", strings.ToUpper(req.Method))) // Ensure uppercase method
		// Get limits from config with fallback values
		limits := r.app.Config().Log.Request.Limits
		uriLimit := limits.URILength
		if uriLimit == 0 {
			uriLimit = 512
		}
		uaLimit := limits.UserAgentLength
		if uaLimit == 0 {
			uaLimit = 256
		}
		refererLimit := limits.RefererLength
		if refererLimit == 0 {
			refererLimit = 512
		}
		ipLimit := limits.RemoteIPLength
		if ipLimit == 0 {
			ipLimit = 64
		}

		attrs = append(attrs, slog.String("uri", cutStr(req.URL.RequestURI(), uriLimit)))
		attrs = append(attrs, slog.Int("status", rec.status))
		attrs = append(attrs, slog.String("duration", duration.String()))
		attrs = append(attrs, slog.String("remote_ip", cutStr(RemoteIP(req), ipLimit)))
		attrs = append(attrs, slog.String("user_agent", cutStr(req.UserAgent(), uaLimit)))
		attrs = append(attrs, slog.String("referer", cutStr(req.Referer(), refererLimit)))
		attrs = append(attrs, slog.String("host", cutStr(req.Host, maxRemoteIP)))
		attrs = append(attrs, slog.String("proto", req.Proto))
		attrs = append(attrs, slog.Int64("content_length", req.ContentLength))
		attrs = append(attrs, emptyAuth)

		// Log request with explicit message
		r.app.Logger().Info("http_request", attrs...)

	})
}

package prerouter

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/caasmo/restinpieces/core"
	"log/slog"
)

const (
	maxBodySize      = 1 << 20 // 1MB
	logMessage   = "http_request"
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


// Execute wraps the next handler with request logging
func (r *RequestLog) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Check if request logging is activated
		if !r.app.Config().Log.Request.Activated {
			next.ServeHTTP(w, req)
			return
		}

		// Limit request body size TODO
		//req.Body = http.MaxBytesReader(w, req.Body, maxBodySize)

		// Check if we already have a ResponseRecorder from the recorder middleware
		rec, ok := w.(*core.ResponseRecorder)
		if !ok {
			r.app.Logger().Error("request log middleware: expected core.ResponseRecorder but got different type",
				"type", "ResponseRecorder",
				"got", w,
			)
			next.ServeHTTP(w, req)
			return
		}
		
		// Call next handler using existing recorder
		next.ServeHTTP(rec, req)
		
		// Get duration from recorder
		duration := rec.Duration()

		// Build log attributes efficiently with cached values and length limits
		attrs := make([]any, 0, 17) 
		attrs = append(attrs, logType)
		attrs = append(attrs, slog.String("method", strings.ToUpper(req.Method))) // Ensure uppercase method
		limits := r.app.Config().Log.Request.Limits
		attrs = append(attrs, slog.String("uri", cutStr(req.URL.RequestURI(), limits.URILength)))
		attrs = append(attrs, slog.Int("status", rec.Status))
		attrs = append(attrs, slog.String("duration", duration.String()))
		attrs = append(attrs, slog.String("remote_ip", cutStr(RemoteIP(req), limits.RemoteIPLength)))
		attrs = append(attrs, slog.String("user_agent", cutStr(req.UserAgent(), limits.UserAgentLength)))
		attrs = append(attrs, slog.String("referer", cutStr(req.Referer(), limits.RefererLength)))
		attrs = append(attrs, slog.String("host", cutStr(req.Host, limits.RemoteIPLength)))
		attrs = append(attrs, slog.String("proto", req.Proto))
		attrs = append(attrs, slog.Bool("tls", req.TLS != nil))
		attrs = append(attrs, slog.Int64("request_content_length", req.ContentLength))
		attrs = append(attrs, emptyAuth)

		r.app.Logger().Info(logMessage, attrs...)

	})
}

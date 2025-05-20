package core

import (
	"net/http"
	"time"
)

// ResponseRecorder is a comprehensive recorder that captures various HTTP response metrics
type ResponseRecorder struct {
	http.ResponseWriter
	Status       int       // HTTP status code
	WroteHeader  bool      // Flag to track if headers were written
	BytesWritten int64     // Total bytes written to response
	StartTime    time.Time // When the request started
	RequestID    string    // Optional request ID for tracing
}

// WriteHeader captures the status code and marks headers as written
func (r *ResponseRecorder) WriteHeader(status int) {
	if !r.WroteHeader {
		r.Status = status
		r.WroteHeader = true
		r.ResponseWriter.WriteHeader(status)
	}
}

// Write captures bytes written and ensures headers are written first
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	// If headers not written yet, call WriteHeader with default status
	if !r.WroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	n, err := r.ResponseWriter.Write(b)
	r.BytesWritten += int64(n)
	return n, err
}

// Duration returns the time elapsed since the request started
func (r *ResponseRecorder) Duration() time.Duration {
	return time.Since(r.StartTime)
}

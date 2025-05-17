package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// BlockRequestBody handles limiting the size of request bodies.
type BlockRequestBody struct {
	app *core.App // Use App to access config
}

// NewBlockRequestBody creates a new request body size limiter middleware instance.
func NewBlockRequestBody(app *core.App) *BlockRequestBody {
	return &BlockRequestBody{
		app: app,
	}
}

// Execute wraps the next handler with request body size limiting logic.
func (l *BlockRequestBody) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// http.MaxBytesReader handles various cases:
		// 1. If Content-Length header exists and is > limitBytes, it immediately rejects.
		// 2. For chunked encoding, or if Content-Length is within limits (or absent),
		//    it wraps r.Body. If reading from r.Body exceeds limitBytes, the Read
		//    operation will fail, and MaxBytesReader sends a 413 response.
		//
		// It's important that MaxBytesReader is set *before* the handler tries to read the body.
		// The server usually makes sure r.Body is non-nil (e.g., http.NoBody for GET).
		// MaxBytesReader handles http.NoBody gracefully.
		r.Body = http.MaxBytesReader(w, r.Body, limitBytes)

		// Call the next handler in the chain.
		// If the next handler (or any subsequent code) tries to read r.Body
		// and exceeds the limit, the Read will fail, and MaxBytesReader
		// will have already sent the 413 response.
		next.ServeHTTP(w, r)

	})
}

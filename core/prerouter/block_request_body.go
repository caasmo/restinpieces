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
		cfg := l.app.Config().BlockRequestBody

		// Skip if middleware is not activated
		if !cfg.Activated {
			next.ServeHTTP(w, r)
			return
		}

		// Check if path is in excluded paths
		for _, path := range cfg.ExcludedPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Apply body size limit
		r.Body = http.MaxBytesReader(w, r.Body, cfg.Limit)

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

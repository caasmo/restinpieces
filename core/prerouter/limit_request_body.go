package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// LimitRequestBody handles limiting the size of request bodies.
type LimitRequestBody struct {
	app *core.App // Use App to access config
}

// NewLimitRequestBody creates a new request body size limiter middleware instance.
func NewLimitRequestBody(app *core.App) *LimitRequestBody {
	return &LimitRequestBody{
		app: app,
	}
}

// Execute wraps the next handler with request body size limiting logic.
func (l *LimitRequestBody) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement request body size limiting
		next.ServeHTTP(w, r)
	})
}

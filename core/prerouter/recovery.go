package prerouter

import (
	"net/http"
	"runtime"

	"github.com/caasmo/restinpieces/core"
)

// Recovery is a prerouter middleware that catches panics raised by the
// handlers and middleware that run after it and answers 500 Internal Server
// Error.
//
// Without this middleware, net/http recovers the panic and writes no
// response; it closes the connection, so the client gets nothing.
//
// Add Recovery first in the prerouter chain so it wraps every other
// middleware and the router.
type Recovery struct {
	app *core.App // Use App to access the logger
}

// NewRecovery creates a new panic recovery middleware instance.
func NewRecovery(app *core.App) *Recovery {
	return &Recovery{
		app: app,
	}
}

// Execute wraps the next handler with panic recovery logic.
func (rc *Recovery) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			panicValue := recover()
			if panicValue == nil {
				return
			}

			// ErrAbortHandler is a sentinel error that aborts the response on
			// purpose: net/http logs no stack trace for it. Re-panic so
			// net/http handles it instead of this middleware logging and
			// answering 500.
			if panicValue == http.ErrAbortHandler {
				panic(panicValue)
			}

			// 2 KB holds about 16 frames. The panic site is the third or
			// fourth frame, so it is always kept; callers past the buffer
			// are cut silently.
			buf := make([]byte, 2048)
			written := runtime.Stack(buf, false)
			stack := buf[:written]

			rc.app.Logger().Error("recovered from panic",
				"panic", panicValue,
				"method", r.Method,
				"uri", r.URL.RequestURI(),
				"remote_ip", rc.app.ClientIP(r),
				"stack", string(stack),
			)

			w.WriteHeader(http.StatusInternalServerError)
		}()

		next.ServeHTTP(w, r)
	})
}

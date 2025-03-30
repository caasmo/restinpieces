//go:build dev

package core

import "net/http"

// StaticHeadersMiddleware returns a middleware that doesn't set any cache headers in development,
// allowing browsers to use their default no-caching behavior.
// Security headers are also omitted to avoid interfering with dev tools.
func StaticHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

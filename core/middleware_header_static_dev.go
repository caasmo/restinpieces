//go:build dev

package core

import "net/http"

// StaticHeadersMiddleware for development doesn't set any cache headers,
// allowing browsers to use their default no-caching behavior.
// Security headers are also omitted to avoid interfering with dev tools.
func StaticHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No headers set - browsers will handle caching as they see fit
		// which typically means no caching during development
		next.ServeHTTP(w, r)
	})
}

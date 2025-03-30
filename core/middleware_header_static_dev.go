//go:build dev

package core

import (
	"net/http"
)

// CacheControlMiddleware adds cache-related HTTP headers suitable for static assets
// during development. It aims to disable caching entirely to ensure developers
// always see the latest changes.
// This version is compiled ONLY when the 'dev' build tag IS specified.
func CacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Apply development caching headers (effectively disabling caching).
		// No need to differentiate file types in dev; we want everything fresh.
		setHeaders(w, headersCacheStaticDev)

		next.ServeHTTP(w, r)
	})
}

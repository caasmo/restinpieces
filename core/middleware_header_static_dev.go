//go:build dev

package core

import (
	"net/http"
)

// StaticHeadersMiddleware adds cache-related HTTP headers suitable for static assets
// during development (dev build tag). It aims to disable caching entirely.
// It does NOT apply the production security headers (like CSP) to avoid interfering
// with development tools (e.g., live reload, browser extensions).
func StaticHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Apply development caching headers (effectively disabling caching).
		// No need to differentiate file types in dev; we want everything fresh.
		// Security headers (like CSP) are intentionally omitted in dev builds.
		setHeaders(w, headersCacheStaticDev)

		next.ServeHTTP(w, r)
	})
}

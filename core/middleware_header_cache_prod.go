//go:build !dev

package core

import (
	"net/http"
)

// CacheControlMiddleware adds cache-related HTTP headers suitable for immutable static assets
// served from an embedded filesystem in a production environment.
// It assumes that assets change only when the application is rebuilt and redeployed.
// This version is compiled when the 'dev' build tag is NOT specified.
func CacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cache-Control directives:
		// - public: Indicates that the response may be cached by any cache (browser, CDN, proxies).
		//           This is appropriate for general static assets like CSS, JS, images.
		// - max-age=31536000: Specifies the maximum time (in seconds) that the resource is
		//                     considered fresh. 31536000 seconds equals 1 year. This tells
		//                     caches they can reuse the asset for a long time without rechecking.
		// - immutable: A strong directive indicating that the response body will not change over time.
		//              If supported, the browser will not attempt to revalidate the resource
		//              (e.g., using If-None-Match or If-Modified-Since), even during a user-initiated
		//              refresh. This is ideal for versioned assets or assets embedded at build time,
		//              as their content is guaranteed not to change until the next application deployment.
		//              Cache busting must be handled by changing the asset's URL (e.g., /app.v2.js).
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		// Note: We are intentionally NOT setting 'Expires' or 'Pragma' headers, as 'Cache-Control'
		// is the modern standard and preferred by HTTP/1.1 and later.
		// Note: We are NOT setting 'ETag' or 'Last-Modified' here. The 'immutable' directive
		// makes them largely redundant for preventing revalidation checks. While ETags could
		// potentially be used by some CDNs or caches even with 'immutable', generating them
		// reliably for embedded assets without performance impact adds complexity. Relying
		// on 'immutable' and URL-based cache busting is simpler and effective for this use case.

		next.ServeHTTP(w, r)
	})
}

//go:build !dev

package core

import (
	"net/http"
	"strings"
)

// CacheControlMiddleware adds cache-related HTTP headers suitable for static assets
// served from an embedded filesystem in a production environment.
// It differentiates between HTML files (which act as entry points and should be revalidated)
// and other assets like CSS, JS, images (which are assumed to be versioned via filename hashing
// and can be cached immutably).
// This version is compiled when the 'dev' build tag is NOT specified.
func CacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Apply appropriate cache headers based on file type using predefined maps.
		if !strings.HasSuffix(r.URL.Path, ".html") {
			// Apply immutable caching headers for non-HTML assets (CSS, JS, images, etc.)
			setHeaders(w, headersCacheStatic)
			next.ServeHTTP(w, r)
			return // Return early for non-HTML assets
		}

		// For HTML files 
		// Apply revalidation caching headers for HTML files.
		setHeaders(w, headersCacheStaticHtml)

		// Note: We intentionally avoid deprecated headers like 'Expires' and 'Pragma'.
		// Note: For immutable assets, 'ETag' and 'Last-Modified' are redundant for
		//       preventing revalidation, so they are not set here for simplicity.
		//       For HTML ('no-cache'), the browser will perform revalidation anyway,
		//       and the server (e.g., http.FileServer) might set ETag/Last-Modified
		//       automatically, which is acceptable.

		next.ServeHTTP(w, r)
	})
}

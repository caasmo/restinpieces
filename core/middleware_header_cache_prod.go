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

		// Determine caching strategy based on file type (inferred from path extension).
		if strings.HasSuffix(r.URL.Path, ".html") {
			// For HTML files:
			// - public: Allows caching by intermediate proxies and browsers.
			// - no-cache: Requires the cache to revalidate with the origin server
			//             before using a cached response. This ensures the user
			//             always gets the latest HTML, which might contain updated
			//             references to versioned assets. It does NOT mean "do not store".
			w.Header().Set("Cache-Control", "public, no-cache")
		} else {
			// For non-HTML assets (CSS, JS, images, fonts, etc.):
			// These are assumed to have versioned filenames (e.g., style.a1b2c3d4.css).
			// - public: Allows caching by intermediate proxies and browsers.
			// - max-age=31536000: Cache for 1 year.
			// - immutable: Indicates the file content will never change. Browsers
			//              will not even attempt to revalidate it, providing maximum
			//              caching efficiency. Relies entirely on filename versioning
			//              for cache busting.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		// Note: We intentionally avoid deprecated headers like 'Expires' and 'Pragma'.
		// Note: For immutable assets, 'ETag' and 'Last-Modified' are redundant for
		//       preventing revalidation, so they are not set here for simplicity.
		//       For HTML ('no-cache'), the browser will perform revalidation anyway,
		//       and the server (e.g., http.FileServer) might set ETag/Last-Modified
		//       automatically, which is acceptable.

		next.ServeHTTP(w, r)
	})
}
		// makes them largely redundant for preventing revalidation checks. While ETags could
		// potentially be used by some CDNs or caches even with 'immutable', generating them
		// reliably for embedded assets without performance impact adds complexity. Relying
		// on 'immutable' and URL-based cache busting is simpler and effective for this use case.

		next.ServeHTTP(w, r)
	})
}

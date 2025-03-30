//go:build !dev

package core

import (
	"net/http"
	"strings"
)

// StaticHeadersMiddleware adds cache and security related HTTP headers suitable for static assets
// served from an embedded filesystem in a production environment (!dev build tag).
// It differentiates between HTML files (applying specific caching and security headers)
// and other assets like CSS, JS, images (applying long-term immutable caching).
func StaticHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// For HTML files:
		if strings.HasSuffix(r.URL.Path, ".html") {
			setHeaders(w, headersCacheStaticHtml)
			setHeaders(w, headersSecurityStaticHtml)
			next.ServeHTTP(w, r)
			return
		}

		// For CSS, JS, images, etc.
		setHeaders(w, headersCacheStatic)

		// Note: We intentionally avoid deprecated headers like 'Expires' and 'Pragma'.
		// Note: For immutable assets, 'ETag' and 'Last-Modified' are redundant for
		//       preventing revalidation, so they are not set here for simplicity.
		//       For HTML ('no-cache'), the browser will perform revalidation anyway,
		//       and the server (e.g., http.FileServer) might set ETag/Last-Modified
		//       automatically, which is acceptable.

		next.ServeHTTP(w, r)
	})
}

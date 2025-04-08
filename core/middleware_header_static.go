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
			setHeaders(w, headersStaticHtml)
			next.ServeHTTP(w, r)
			return
		}

		// For CSS, JS, images, etc.
		setHeaders(w, headersStatic)

		// Note: We intentionally avoid deprecated headers like 'Expires' and 'Pragma'.
		// Note: For immutable assets, 'ETag' and 'Last-Modified' are redundant for

		next.ServeHTTP(w, r)
	})
}

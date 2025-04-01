package core

import "net/http"

// faviconHandler handles requests for /favicon.ico by returning a 204 No Content.
// This prevents 404 errors in logs for browsers that automatically request it.
// It avoids serving an actual icon file, keeping the API server focused.
// Caching headers are applied by the middleware or router configuration.
func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	setHeaders(w, HeadersFavicon)
	w.WriteHeader(http.StatusNoContent)
}

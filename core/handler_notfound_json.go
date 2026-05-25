package core

import "net/http"

// NotFoundJSONHandler writes a precomputed 404 JSON error response.
// It is intended for API routes where a JSON error is preferred over the default text 404.
func NotFoundJSONHandler(w http.ResponseWriter, r *http.Request) {
	WriteJsonError(w, errorNotFound)
}

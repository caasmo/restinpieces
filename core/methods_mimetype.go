package core

import (
	"net/http"
	"strings"
)

// ValidateContentType checks if the request's Content-Type matches the allowed type.
// Returns nil if the content type is valid, otherwise returns a precomputed error response.
// Uses http.StatusUnsupportedMediaType (415) for invalid content types as per HTTP spec.
func (a *App) ValidateContentType(r *http.Request, allowedType string) jsonResponse {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return errorInvalidContentType
	}

	// Handle cases where Content-Type includes charset or other parameters
	// e.g. "application/json; charset=utf-8"
	mediaType := strings.Split(contentType, ";")[0]
	mediaType = strings.TrimSpace(mediaType)

	if mediaType != allowedType {
		return errorInvalidContentType
	}

	return jsonResponse{}
}

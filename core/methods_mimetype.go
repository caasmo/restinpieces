package core

import (
	"net/http"
	"strings"
	"errors"
)

// ValidateContentType checks if the request's Content-Type matches the allowed type.
// Returns:
// - error (always "Invalid content type" for security)
// - precomputed jsonResponse for error cases
// Uses http.StatusUnsupportedMediaType (415) for invalid content types as per HTTP spec.
func (a *App) ValidateContentType(r *http.Request, allowedType string) (error, jsonResponse) {
	errInvalidType := errors.New("Invalid content type")
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return errInvalidType, errorInvalidContentType
	}

	// Handle cases where Content-Type includes charset or other parameters
	// e.g. "application/json; charset=utf-8"
	mediaType := strings.Split(contentType, ";")[0]
	mediaType = strings.TrimSpace(mediaType)

	if mediaType != allowedType {
		return errInvalidType, errorInvalidContentType
	}

	return nil, jsonResponse{}
}

package app

import (
	"fmt"
	"net/http"
)

type jsonError struct {
	code int
	body []byte
}

var jsonHeader = []string{"application/json; charset=utf-8"}

// Precomputed error responses with status codes
var (
	errorTokenGeneration      = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
	errorClaimsNotFound       = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token: Claims not found"}`)}
	errorInvalidRequest       = jsonError{http.StatusBadRequest, []byte(`{"error":"Invalid request payload"}`)}
	errorInvalidCredentials   = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid credentials"}`)}
	errorPasswordMismatch     = jsonError{http.StatusBadRequest, []byte(`{"error":"Password and confirmation do not match"}`)}
	errorMissingFields        = jsonError{http.StatusBadRequest, []byte(`{"error":"Missing required fields"}`)}
	errorPasswordComplexity   = jsonError{http.StatusBadRequest, []byte(`{"error":"Password must be at least 8 characters"}`)}
	errorEmailConflict        = jsonError{http.StatusConflict, []byte(`{"error":"Email already registered"}`)}
	errorNotFound             = jsonError{http.StatusNotFound, []byte(`{"error":"Email not found"}`)}
	errorConflict             = jsonError{http.StatusConflict, []byte(`{"error":"Verification already requested"}`)}
	errorRegistrationFailed   = jsonError{http.StatusBadRequest, []byte(`{"error":"Registration failed"}`)}
	errorTooManyRequests      = jsonError{http.StatusTooManyRequests, []byte(`{"error":"Too many requests"}`)}
	errorServiceUnavailable   = jsonError{http.StatusServiceUnavailable, []byte(`{"error":"Service unavailable"}`)}
	errorNoAuthHeader         = jsonError{http.StatusUnauthorized, []byte(`{"error":"Authorization header required"}`)}
	errorInvalidTokenFormat   = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid authorization format"}`)}
	errorJwtInvalidSignMethod = jsonError{http.StatusUnauthorized, []byte(`{"error":"unexpected signing method"}`)}
	errorJwtTokenExpired      = jsonError{http.StatusUnauthorized, []byte(`{"error":"Token expired"}`)}
	errorAlreadyVerified      = jsonError{http.StatusConflict, []byte(`{"error":"Already verified"}`)}
	errorJwtInvalidToken      = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid token"}`)}
)

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.code)
	w.Write(err.body)
}

// writeJSONErrorf writes a formatted JSON error response
func writeJSONErrorf(w http.ResponseWriter, code int, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	fmt.Fprintf(w, format, args...)
}

// writeOAuth2TokenResponse writes a standardized OAuth2 token response
func writeOAuth2TokenResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token_type": "Bearer",
		"access_token": token,
		"expires_in": expiresIn,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}

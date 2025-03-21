package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonError struct {
	code    int
	status  string
	message string
}

var jsonHeader = []string{"application/json; charset=utf-8"}

// Standard error codes and messages
const (
	CodeTokenGeneration      = "token_generation_error"
	CodeClaimsNotFound       = "claims_not_found"
	CodeInvalidRequest       = "invalid_input"
	CodeInvalidCredentials   = "invalid_credentials"
	CodePasswordMismatch     = "password_mismatch"
	CodeMissingFields        = "missing_fields"
	CodePasswordComplexity   = "password_complexity"
	CodeEmailConflict        = "email_conflict"
	CodeNotFound             = "not_found"
	CodeConflict             = "conflict"
	CodeRegistrationFailed   = "registration_failed"
	CodeTooManyRequests      = "too_many_requests"
	CodeServiceUnavailable   = "service_unavailable"
	CodeNoAuthHeader         = "no_auth_header"
	CodeInvalidTokenFormat   = "invalid_token_format"
	CodeJwtInvalidSignMethod = "invalid_sign_method"
	CodeJwtTokenExpired      = "token_expired"
	CodeAlreadyVerified      = "already_verified"
	CodeJwtInvalidToken      = "invalid_token"
)

// Precomputed error responses with status codes
var (
	errorTokenGeneration      = jsonError{http.StatusInternalServerError, CodeTokenGeneration, "Failed to generate authentication token"}
	errorClaimsNotFound       = jsonError{http.StatusInternalServerError, CodeClaimsNotFound, "Failed to generate token: Claims not found"}
	errorInvalidRequest       = jsonError{http.StatusBadRequest, CodeInvalidRequest, "The request contains invalid data"}
	errorInvalidCredentials   = jsonError{http.StatusUnauthorized, CodeInvalidCredentials, "Invalid credentials provided"}
	errorPasswordMismatch     = jsonError{http.StatusBadRequest, CodePasswordMismatch, "Password and confirmation do not match"}
	errorMissingFields        = jsonError{http.StatusBadRequest, CodeMissingFields, "Required fields are missing"}
	errorPasswordComplexity   = jsonError{http.StatusBadRequest, CodePasswordComplexity, "Password must be at least 8 characters"}
	errorEmailConflict        = jsonError{http.StatusConflict, CodeEmailConflict, "Email address is already registered"}
	errorNotFound             = jsonError{http.StatusNotFound, CodeNotFound, "Requested resource not found"}
	errorConflict             = jsonError{http.StatusConflict, CodeConflict, "Verification already requested"}
	errorRegistrationFailed   = jsonError{http.StatusBadRequest, CodeRegistrationFailed, "Registration failed due to invalid data"}
	errorTooManyRequests      = jsonError{http.StatusTooManyRequests, CodeTooManyRequests, "Too many requests, please try again later"}
	errorServiceUnavailable   = jsonError{http.StatusServiceUnavailable, CodeServiceUnavailable, "Service is temporarily unavailable"}
	errorNoAuthHeader         = jsonError{http.StatusUnauthorized, CodeNoAuthHeader, "Authorization header is required"}
	errorInvalidTokenFormat   = jsonError{http.StatusUnauthorized, CodeInvalidTokenFormat, "Invalid authorization token format"}
	errorJwtInvalidSignMethod = jsonError{http.StatusUnauthorized, CodeJwtInvalidSignMethod, "Invalid JWT signing method"}
	errorJwtTokenExpired      = jsonError{http.StatusUnauthorized, CodeJwtTokenExpired, "Authentication token has expired"}
	errorAlreadyVerified      = jsonError{http.StatusConflict, CodeAlreadyVerified, "Account is already verified"}
	errorJwtInvalidToken      = jsonError{http.StatusUnauthorized, CodeJwtInvalidToken, "Invalid authentication token"}
)

// writeJSONError writes a standardized JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  err.code,
		"code":    err.status,
		"message": err.message,
	})
}

// writeJSONErrorf writes a formatted JSON error response with custom message
func writeJSONErrorf(w http.ResponseWriter, code int, status string, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  code,
		"code":    status,
		"message": fmt.Sprintf(format, args...),
	})
}

// writeAuthTokenResponse writes a standardized authentication token response
// Used for both password and OAuth2 authentication
func writeAuthTokenResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
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

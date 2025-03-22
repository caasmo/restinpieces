package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonError struct {
	status int
	body   []byte
}

var jsonHeader = []string{"application/json; charset=utf-8"}

// Standard error codes and messages
const (
	CodeTokenGeneration      = "token_generation"
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

// precomputeError() will be executed during initialization (before main() runs),
// and the JSON body will be precomputed and stored in the error variables.
// the variables will contain the fully JSON as []byte already
// It avoids repeated JSON marshaling during request handling
// Any time we use writeJSONError(w, errorTokenGeneration) in the code, it
// simply writes the pre-computed bytes to the response writer
func precomputeError(status int, code, message string) jsonError {
	body := fmt.Sprintf(`{"status":%d,"code":"%s","message":"%s"}`, status, code, message)
	return jsonError{status: status, body: []byte(body)}
}

// Precomputed error responses with status codes
var (
	errorTokenGeneration      = precomputeError(http.StatusInternalServerError, CodeTokenGeneration, "Failed to generate authentication token")
	errorClaimsNotFound       = precomputeError(http.StatusInternalServerError, CodeClaimsNotFound, "Failed to generate token: Claims not found")
	errorInvalidRequest       = precomputeError(http.StatusBadRequest, CodeInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials   = precomputeError(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch     = precomputeError(http.StatusBadRequest, CodePasswordMismatch, "Password and confirmation do not match")
	errorMissingFields        = precomputeError(http.StatusBadRequest, CodeMissingFields, "Required fields are missing")
	errorPasswordComplexity   = precomputeError(http.StatusBadRequest, CodePasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict        = precomputeError(http.StatusConflict, CodeEmailConflict, "Email address is already registered")
	errorNotFound             = precomputeError(http.StatusNotFound, CodeNotFound, "Requested resource not found")
	errorConflict             = precomputeError(http.StatusConflict, CodeConflict, "Verification already requested")
	errorRegistrationFailed   = precomputeError(http.StatusBadRequest, CodeRegistrationFailed, "Registration failed due to invalid data")
	errorTooManyRequests      = precomputeError(http.StatusTooManyRequests, CodeTooManyRequests, "Too many requests, please try again later")
	errorServiceUnavailable   = precomputeError(http.StatusServiceUnavailable, CodeServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader         = precomputeError(http.StatusUnauthorized, CodeNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat   = precomputeError(http.StatusUnauthorized, CodeInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod = precomputeError(http.StatusUnauthorized, CodeJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired      = precomputeError(http.StatusUnauthorized, CodeJwtTokenExpired, "Authentication token has expired")
	errorAlreadyVerified      = precomputeError(http.StatusConflict, CodeAlreadyVerified, "Account is already verified")
	errorJwtInvalidToken      = precomputeError(http.StatusUnauthorized, CodeJwtInvalidToken, "Invalid authentication token")
)

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.status)
	w.Write(err.body)
}

// writeJSONErrorf writes a formatted JSON error response with custom message
func writeJSONErrorf(w http.ResponseWriter, status int, code string, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  status,
		"code":    code,
		"message": fmt.Sprintf(format, args...),
	})
}

// writeAuthTokenResponse writes a standardized authentication token response
// Used for both password and OAuth2 authentication
func writeAuthTokenResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token_type":   "Bearer",
		"access_token": token,
		"expires_in":   expiresIn,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}

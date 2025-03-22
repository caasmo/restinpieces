package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonResponse struct {
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
	CodeJwtInvalidVerificationToken= "invalid_verification_token"
)

// precomputeResponse() will be executed during initialization (before main() runs),
// and the JSON body will be precomputed and stored in the response variables.
// the variables will contain the fully JSON as []byte already
// It avoids repeated JSON marshaling during request handling
// Any time we use writeJSONResponse(w, response) in the code, it
// simply writes the pre-computed bytes to the response writer
func precomputeResponse(status int, code, message string) jsonResponse {
	body := fmt.Sprintf(`{"status":%d,"code":"%s","message":"%s"}`, status, code, message)
	return jsonResponse{status: status, body: []byte(body)}
}

// For successful responses
func writeJSONOk(w http.ResponseWriter, status int, code, message string) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"status":%d,"code":"%s","message":"%s"}`, status, code, message)
}

// Precomputed error amd ok responses with status codes
var (
	//errors
	errorTokenGeneration      = precomputeResponse(http.StatusInternalServerError, CodeTokenGeneration, "Failed to generate authentication token")
	errorClaimsNotFound       = precomputeResponse(http.StatusInternalServerError, CodeClaimsNotFound, "Failed to generate token: Claims not found")
	errorInvalidRequest       = precomputeResponse(http.StatusBadRequest, CodeInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials   = precomputeResponse(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch     = precomputeResponse(http.StatusBadRequest, CodePasswordMismatch, "Password and confirmation do not match")
	errorMissingFields        = precomputeResponse(http.StatusBadRequest, CodeMissingFields, "Required fields are missing")
	errorPasswordComplexity   = precomputeResponse(http.StatusBadRequest, CodePasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict        = precomputeResponse(http.StatusConflict, CodeEmailConflict, "Email address is already registered")
	errorNotFound             = precomputeResponse(http.StatusNotFound, CodeNotFound, "Requested resource not found")
	errorConflict             = precomputeResponse(http.StatusConflict, CodeConflict, "Verification already requested")
	errorRegistrationFailed   = precomputeResponse(http.StatusBadRequest, CodeRegistrationFailed, "Registration failed due to invalid data")
	errorTooManyRequests      = precomputeResponse(http.StatusTooManyRequests, CodeTooManyRequests, "Too many requests, please try again later")
	errorServiceUnavailable   = precomputeResponse(http.StatusServiceUnavailable, CodeServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader         = precomputeResponse(http.StatusUnauthorized, CodeNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat   = precomputeResponse(http.StatusUnauthorized, CodeInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired      = precomputeResponse(http.StatusUnauthorized, CodeJwtTokenExpired, "Authentication token has expired")
	errorJwtInvalidToken      = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidToken, "Invalid authentication token")
	errorJwtInvalidVerificationToken      = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidVerificationToken, "Invalid verification token")
	errorEmailVerificationFailed          = precomputeResponse(http.StatusInternalServerError, "email_verification_failed", "Email verification process failed")

	// oks
	okAlreadyVerified         = precomputeResponse(http.StatusAccepted, "already_verified", "Email already verified - no further action needed")
)

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, resp jsonResponse) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(resp.status)
	w.Write(resp.body)
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
